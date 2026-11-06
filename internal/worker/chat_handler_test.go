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
	"strings"
	"testing"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/notification"
	"github.com/hibiken/asynq"
)

type fakeChatStore struct {
	conv  *models.ChatConversation
	first *models.ChatMessage
}

func (f *fakeChatStore) FindConversationByID(uint) (*models.ChatConversation, error) {
	return f.conv, nil
}
func (f *fakeChatStore) FirstMessage(uint) (*models.ChatMessage, error) { return f.first, nil }
func (f *fakeChatStore) FindWidgetByID(uint) (*models.ChatWidget, error) {
	return &models.ChatWidget{UUID: "w-uuid", DisplayName: "Website chat"}, nil
}
func (f *fakeChatStore) FindMessageByID(uint) (*models.ChatMessage, error) { return f.first, nil }

type sentNotice struct {
	userID      uint
	workspaceID uint
	subject     string
	template    string
	data        map[string]any
}

type fakeChatNotifier struct{ sent []sentNotice }

func (f *fakeChatNotifier) IsConfigured() bool { return true }
func (f *fakeChatNotifier) SendToUser(user uint, subject, tmpl string, data map[string]any) error {
	f.sent = append(f.sent, sentNotice{workspaceID: 0, subject: subject, template: tmpl, data: data, userID: user})
	return nil
}
func (f *fakeChatNotifier) SendToWorkspaceAdmins(ws uint, subject, tmpl string, data map[string]any) error {
	f.sent = append(f.sent, sentNotice{workspaceID: ws, subject: subject, template: tmpl, data: data})
	return nil
}

func chatNotifyTask(t *testing.T) *asynq.Task {
	t.Helper()
	payload, _ := json.Marshal(ChatNotifyPayload{ConversationID: 1})
	return asynq.NewTask(TypeChatNotify, payload)
}

func TestChatNotifyTellsAdmins(t *testing.T) {
	ws := uint(3)
	store := &fakeChatStore{
		conv: &models.ChatConversation{
			ID: 1, UUID: "c-uuid", WorkspaceID: &ws, Status: models.MessageStatusReceived, State: models.ChatStateOpen,
			StartPageURL: "https://example.com/pricing",
			Visitor:      &models.ChatVisitor{Name: "Jane", Email: "jane@example.com"},
			Widget:       &models.ChatWidgetRef{DisplayName: "Website chat"},
		},
		first: &models.ChatMessage{Body: "Do you ship to Canada?"},
	}
	notifier := &fakeChatNotifier{}
	h := &ChatNotifyHandler{repo: store, notifier: notifier, appURL: "https://posta.test"}
	if err := h.ProcessTask(context.Background(), chatNotifyTask(t)); err != nil {
		t.Fatal(err)
	}
	if len(notifier.sent) != 1 {
		t.Fatalf("sent %d notifications, want 1", len(notifier.sent))
	}
	n := notifier.sent[0]
	if n.workspaceID != ws || n.template != notification.TemplateNewChat || n.subject != "New chat from Jane" {
		t.Fatalf("notification = %+v", n)
	}
	if n.data["ConversationURL"] != "https://posta.test/chat/c-uuid" || n.data["Body"] != "Do you ship to Canada?" {
		t.Fatalf("data = %+v", n.data)
	}
}

func TestChatNotifySkipsQuarantinedAndSpam(t *testing.T) {
	ws := uint(3)
	for _, conv := range []*models.ChatConversation{
		{ID: 1, WorkspaceID: &ws, Status: models.MessageStatusQuarantined, State: models.ChatStateOpen},
		{ID: 1, WorkspaceID: &ws, Status: models.MessageStatusReceived, State: models.ChatStateSpam},
	} {
		notifier := &fakeChatNotifier{}
		h := &ChatNotifyHandler{repo: &fakeChatStore{conv: conv, first: &models.ChatMessage{}}, notifier: notifier}
		if err := h.ProcessTask(context.Background(), chatNotifyTask(t)); err != nil {
			t.Fatal(err)
		}
		if len(notifier.sent) != 0 {
			t.Fatalf("notified for a %s/%s conversation", conv.Status, conv.State)
		}
	}
}

type fakePresence bool

func (p fakePresence) AnyOnline(context.Context, uint) bool { return bool(p) }

func TestChatNotifySkipsEmailWhenAnAgentIsOnline(t *testing.T) {
	ws := uint(3)
	notifier := &fakeChatNotifier{}
	h := &ChatNotifyHandler{
		repo:     &fakeChatStore{conv: &models.ChatConversation{ID: 1, WorkspaceID: &ws, State: models.ChatStateOpen}, first: &models.ChatMessage{}},
		notifier: notifier, presence: fakePresence(true),
	}
	if err := h.ProcessTask(context.Background(), chatNotifyTask(t)); err != nil {
		t.Fatal(err)
	}
	if len(notifier.sent) != 0 {
		t.Fatal("emailed admins although an agent was online to see the chat")
	}
}

func unansweredTask(seq int64) *asynq.Task {
	payload, _ := json.Marshal(ChatMessagePayload{ConversationID: 1, Seq: seq})
	return asynq.NewTask(TypeChatUnanswered, payload)
}

func TestChatUnansweredAlertsTheAssigneeOnce(t *testing.T) {
	ws, agent := uint(3), uint(9)
	conv := &models.ChatConversation{ID: 1, UUID: "c", WorkspaceID: &ws, State: models.ChatStateOpen,
		LastAuthorKind: models.ChatAuthorVisitor, LastSeq: 4, AssignedToID: &agent, LastMessagePreview: "Hello?"}
	notifier := &fakeChatNotifier{}
	seen := map[string]bool{}
	h := &ChatNotifyHandler{repo: &fakeChatStore{conv: conv}, notifier: notifier,
		once: func(_ context.Context, key string, _ time.Duration) bool {
			if seen[key] {
				return false
			}
			seen[key] = true
			return true
		}}

	// An older visitor message: a later task covers it.
	_ = h.ProcessUnanswered(context.Background(), unansweredTask(3))
	if len(notifier.sent) != 0 {
		t.Fatal("alerted for a message that is no longer the newest")
	}
	_ = h.ProcessUnanswered(context.Background(), unansweredTask(4))
	if len(notifier.sent) != 1 || notifier.sent[0].userID != agent || notifier.sent[0].template != notification.TemplateChatUnanswered {
		t.Fatalf("sent = %+v, want one alert to the assignee", notifier.sent)
	}
	// Within the hour: debounced.
	_ = h.ProcessUnanswered(context.Background(), unansweredTask(4))
	if len(notifier.sent) != 1 {
		t.Fatal("alerted twice within the debounce window")
	}
}

func TestStillUnanswered(t *testing.T) {
	ws := uint(1)
	base := models.ChatConversation{WorkspaceID: &ws, State: models.ChatStateOpen, LastAuthorKind: models.ChatAuthorVisitor, LastSeq: 5}
	cases := []struct {
		name string
		edit func(c *models.ChatConversation)
		want bool
	}{
		{"waiting", func(*models.ChatConversation) {}, true},
		{"agent answered", func(c *models.ChatConversation) { c.LastAuthorKind = models.ChatAuthorAgent; c.LastSeq = 6 }, false},
		{"closed", func(c *models.ChatConversation) { c.State = models.ChatStateClosed }, false},
		{"quarantined", func(c *models.ChatConversation) { c.Status = models.MessageStatusQuarantined }, false},
	}
	for _, tc := range cases {
		c := base
		tc.edit(&c)
		if got := stillUnanswered(&c, 5); got != tc.want {
			t.Errorf("%s: stillUnanswered = %v, want %v", tc.name, got, tc.want)
		}
	}
}

type fakeDispatch struct {
	event string
	ws    *uint
	body  []byte
}

func (f *fakeDispatch) DispatchJSON(_ uint, ws *uint, event string, body []byte, _ string) {
	f.event, f.ws, f.body = event, ws, body
}
func (f *fakeDispatch) DispatchJSONTo(uint, []uint, string, []byte) {}

func TestChatWebhookPayload(t *testing.T) {
	ws := uint(3)
	store := &fakeChatStore{
		conv: &models.ChatConversation{ID: 1, UUID: "c-uuid", WorkspaceID: &ws, State: models.ChatStateOpen, Status: models.MessageStatusReceived,
			Visitor: &models.ChatVisitor{UUID: "v-uuid", Name: "Jane", Email: "jane@acme.test", EmailVerified: true, ExternalID: "u_1", TokenHash: "secret-hash"}},
		first: &models.ChatMessage{UUID: "m-uuid", Seq: 2, AuthorKind: models.ChatAuthorVisitor, Body: "hello", Channel: "chat",
			Attachments: models.ChatAttachments{{Filename: "a.png", ContentType: "image/png", Size: 10, Key: "chat/1/blob-key"}}},
	}
	d := &fakeDispatch{}
	h := &ChatNotifyHandler{repo: store, dispatcher: d}
	payload, _ := json.Marshal(ChatWebhookPayload{Event: "chat.message.created", ConversationID: 1, MessageID: 7})
	if err := h.ProcessWebhook(context.Background(), asynq.NewTask(TypeChatWebhook, payload)); err != nil {
		t.Fatal(err)
	}
	if d.event != "chat.message.created" || d.ws == nil || *d.ws != ws {
		t.Fatalf("dispatched %q to %v", d.event, d.ws)
	}
	body := string(d.body)
	for _, want := range []string{`"id":"c-uuid"`, `"widget_id":"w-uuid"`, `"email_verified":true`, `"body":"hello"`, `"filename":"a.png"`} {
		if !strings.Contains(body, want) {
			t.Errorf("payload lacks %s: %s", want, body)
		}
	}
	for _, leak := range []string{"secret-hash", "blob-key", "token"} {
		if strings.Contains(body, leak) {
			t.Errorf("payload leaks %q: %s", leak, body)
		}
	}
}
