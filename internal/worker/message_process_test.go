// Copyright (c) 2026 Jonas Kaninda.
//
// This file is part of Posta.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/messages"
	"github.com/goposta/posta/internal/services/realtime"
	"github.com/hibiken/asynq"
)

type fakeMessageStore struct {
	msg      *models.Message
	reply    *models.MessageReply
	updates  []map[string]any
	notified int
}

func (f *fakeMessageStore) FindByID(uint) (*models.Message, error) {
	if f.msg == nil {
		return nil, errors.New("not found")
	}
	cp := *f.msg
	return &cp, nil
}

func (f *fakeMessageStore) UpdateFields(_ uint, fields map[string]any) error {
	f.updates = append(f.updates, fields)
	if v, ok := fields["webhook_dispatched_at"].(time.Time); ok {
		f.msg.WebhookDispatchedAt = &v
	}
	return nil
}

func (f *fakeMessageStore) MarkNotified(_ []uint, at time.Time) error {
	f.notified++
	f.msg.NotifiedAt = &at
	return nil
}

func (f *fakeMessageStore) FindReplyByID(uint) (*models.MessageReply, error) {
	if f.reply == nil {
		return nil, errors.New("not found")
	}
	return f.reply, nil
}

type fakeFormStore struct{ form *models.Form }

func (f fakeFormStore) FindByID(uint) (*models.Form, error) { return f.form, nil }

type fakeDispatcher struct{ events []string }

func (f *fakeDispatcher) DispatchJSON(_ uint, _ *uint, event string, _ []byte, _ string) {
	f.events = append(f.events, event)
}

type fakeNotifier struct {
	err  error
	sent []string // "to:<addr>", "user", or "admins"
}

func (f *fakeNotifier) IsConfigured() bool { return true }
func (f *fakeNotifier) Send(to, _, _ string, _ map[string]any) error {
	f.sent = append(f.sent, "to:"+to)
	return f.err
}
func (f *fakeNotifier) SendToUser(userID uint, _, _ string, _ map[string]any) error {
	f.sent = append(f.sent, "user")
	return f.err
}
func (f *fakeNotifier) SendToWorkspaceAdmins(uint, string, string, map[string]any) error {
	f.sent = append(f.sent, "admins")
	return f.err
}

func newTestHandler(store *fakeMessageStore, form *models.Form) (*MessageProcessHandler, *fakeDispatcher, *fakeNotifier) {
	d, n := &fakeDispatcher{}, &fakeNotifier{}
	h := &MessageProcessHandler{
		messageRepo: store,
		formRepo:    fakeFormStore{form: form},
		dispatcher:  d,
		notifier:    n,
		retryCount:  func(context.Context) int { return 0 },
	}
	return h, d, n
}

// processTask builds the message:process task for message 1, the ID every
// fakeMessageStore fixture uses.
func processTask(t *testing.T) *asynq.Task {
	t.Helper()
	payload, _ := json.Marshal(MessageProcessPayload{MessageID: 1})
	return asynq.NewTask(TypeMessageProcess, payload)
}

func immediateForm() *models.Form {
	return &models.Form{ID: 1, Name: "Contact", NotifyEnabled: true, NotifyMode: models.NotifyModeImmediate, NotifyOnFlagged: true}
}

func TestProcessTaskReturnsNotifyErrorSoAsynqRetries(t *testing.T) {
	ws := uint(3)
	store := &fakeMessageStore{msg: &models.Message{ID: 1, FormID: 1, WorkspaceID: &ws, Status: models.MessageStatusReceived}}
	h, _, n := newTestHandler(store, immediateForm())
	n.err = errors.New("smtp down")

	if err := h.ProcessTask(context.Background(), processTask(t)); err == nil {
		t.Fatal("a failed notification must fail the task so it is retried")
	}
	if store.notified != 0 {
		t.Fatal("notified_at must not be stamped when the notification failed")
	}
}

func TestProcessTaskRetryDoesNotRedispatchWebhook(t *testing.T) {
	ws := uint(3)
	store := &fakeMessageStore{msg: &models.Message{ID: 1, FormID: 1, WorkspaceID: &ws, Status: models.MessageStatusReceived}}
	h, d, n := newTestHandler(store, immediateForm())

	n.err = errors.New("smtp down")
	_ = h.ProcessTask(context.Background(), processTask(t))
	n.err = nil
	if err := h.ProcessTask(context.Background(), processTask(t)); err != nil {
		t.Fatalf("retry: %v", err)
	}

	if len(d.events) != 1 || d.events[0] != "message.received" {
		t.Fatalf("webhook events = %v, want exactly one message.received", d.events)
	}
	if store.notified != 1 {
		t.Fatalf("notified %d times, want 1", store.notified)
	}
}

func TestProcessTaskPublishesReceivedToRealtime(t *testing.T) {
	ws := uint(3)
	store := &fakeMessageStore{msg: &models.Message{ID: 1, UUID: "m-1", FormID: 1, WorkspaceID: &ws, Status: models.MessageStatusReceived}}
	h, _, _ := newTestHandler(store, immediateForm())
	bus := realtime.NewMemory()
	h.SetRealtime(bus)
	ch, unsub := bus.Subscribe(context.Background(), realtime.WorkspaceTopic(ws))
	defer unsub()

	if err := h.ProcessTask(context.Background(), processTask(t)); err != nil {
		t.Fatal(err)
	}
	select {
	case ev := <-ch:
		if ev.Type != messages.EventReceived || ev.WorkspaceID != ws {
			t.Fatalf("event = %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("no message.received event published")
	}
}

func TestProcessTaskQuarantinedFiresSpamWebhookWithoutNotifying(t *testing.T) {
	ws := uint(3)
	store := &fakeMessageStore{msg: &models.Message{ID: 1, FormID: 1, WorkspaceID: &ws, Status: models.MessageStatusQuarantined}}
	h, d, n := newTestHandler(store, immediateForm())

	if err := h.ProcessTask(context.Background(), processTask(t)); err != nil {
		t.Fatal(err)
	}
	if len(d.events) != 1 || d.events[0] != "message.spam" {
		t.Fatalf("webhook events = %v", d.events)
	}
	if len(n.sent) != 0 {
		t.Fatalf("quarantined message notified: %v", n.sent)
	}
}

// replyTask builds the message:reply task for reply 5, the ID every reply
// fixture uses.
func replyTask(t *testing.T) *asynq.Task {
	t.Helper()
	payload, _ := json.Marshal(MessageReplyPayload{ReplyID: 5})
	return asynq.NewTask(TypeMessageReply, payload)
}

func TestReplyTaskInboundNotifiesAssignee(t *testing.T) {
	ws, assignee := uint(3), uint(9)
	store := &fakeMessageStore{
		msg:   &models.Message{ID: 1, FormID: 1, WorkspaceID: &ws, AssignedToID: &assignee, Status: models.MessageStatusReceived},
		reply: &models.MessageReply{ID: 5, MessageID: 1, Kind: models.MessageReplyKindInbound, FromAddr: "ada@example.com"},
	}
	h, d, n := newTestHandler(store, immediateForm())

	if err := h.ProcessReplyTask(context.Background(), replyTask(t)); err != nil {
		t.Fatal(err)
	}
	if len(d.events) != 1 || d.events[0] != messages.EventReplyReceived {
		t.Fatalf("webhook events = %v", d.events)
	}
	if len(n.sent) != 1 || n.sent[0] != "user" {
		t.Fatalf("notifications = %v, want the assignee", n.sent)
	}
}

func TestReplyTaskInboundWithoutAssigneeUsesFormRecipients(t *testing.T) {
	ws := uint(3)
	form := immediateForm()
	form.NotifyEmails = []string{"team@example.com"}
	store := &fakeMessageStore{
		msg:   &models.Message{ID: 1, FormID: 1, WorkspaceID: &ws, Status: models.MessageStatusReceived},
		reply: &models.MessageReply{ID: 5, MessageID: 1, Kind: models.MessageReplyKindInbound},
	}
	h, _, n := newTestHandler(store, form)

	if err := h.ProcessReplyTask(context.Background(), replyTask(t)); err != nil {
		t.Fatal(err)
	}
	if len(n.sent) != 1 || n.sent[0] != "to:team@example.com" {
		t.Fatalf("notifications = %v", n.sent)
	}
}

func TestReplyTaskOperatorFiresRepliedWithoutNotifying(t *testing.T) {
	ws := uint(3)
	store := &fakeMessageStore{
		msg:   &models.Message{ID: 1, FormID: 1, WorkspaceID: &ws, Status: models.MessageStatusReceived},
		reply: &models.MessageReply{ID: 5, MessageID: 1, Kind: models.MessageReplyKindOperator},
	}
	h, d, n := newTestHandler(store, immediateForm())

	if err := h.ProcessReplyTask(context.Background(), replyTask(t)); err != nil {
		t.Fatal(err)
	}
	if len(d.events) != 1 || d.events[0] != messages.EventReplied {
		t.Fatalf("webhook events = %v", d.events)
	}
	if len(n.sent) != 0 {
		t.Fatalf("operator reply notified: %v", n.sent)
	}
}

func TestReplyTaskRetrySkipsWebhook(t *testing.T) {
	ws := uint(3)
	store := &fakeMessageStore{
		msg:   &models.Message{ID: 1, FormID: 1, WorkspaceID: &ws, Status: models.MessageStatusReceived},
		reply: &models.MessageReply{ID: 5, MessageID: 1, Kind: models.MessageReplyKindInbound},
	}
	h, d, _ := newTestHandler(store, immediateForm())
	h.retryCount = func(context.Context) int { return 1 }

	if err := h.ProcessReplyTask(context.Background(), replyTask(t)); err != nil {
		t.Fatal(err)
	}
	if len(d.events) != 0 {
		t.Fatalf("a retry re-fired the webhook: %v", d.events)
	}
}

func TestReplyTaskSkipsSpamThreads(t *testing.T) {
	ws := uint(3)
	store := &fakeMessageStore{
		msg:   &models.Message{ID: 1, FormID: 1, WorkspaceID: &ws, Status: models.MessageStatusQuarantined},
		reply: &models.MessageReply{ID: 5, MessageID: 1, Kind: models.MessageReplyKindInbound},
	}
	h, _, n := newTestHandler(store, immediateForm())

	if err := h.ProcessReplyTask(context.Background(), replyTask(t)); err != nil {
		t.Fatal(err)
	}
	if len(n.sent) != 0 {
		t.Fatalf("a reply on a spam thread notified: %v", n.sent)
	}
}
