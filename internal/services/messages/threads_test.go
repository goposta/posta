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

package messages

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/realtime"
	"gorm.io/gorm"
)

func TestThreadHeadersFirstFollowUp(t *testing.T) {
	inReplyTo, refs := threadHeaders("<root@x>", nil)
	if inReplyTo != "<root@x>" || refs != "<root@x>" {
		t.Fatalf("got %q / %q", inReplyTo, refs)
	}
}

func TestThreadHeadersChainsToLatestReply(t *testing.T) {
	inReplyTo, refs := threadHeaders("<root@x>", []string{"<root@x>", "<a@x>", "<b@x>"})
	if inReplyTo != "<b@x>" {
		t.Fatalf("In-Reply-To = %q, want the latest reply", inReplyTo)
	}
	if refs != "<root@x> <a@x> <b@x>" {
		t.Fatalf("References = %q", refs)
	}
}

func TestThreadHeadersCapsReferences(t *testing.T) {
	var prior []string
	for i := 0; i < 25; i++ {
		prior = append(prior, fmt.Sprintf("<r%d@x>", i))
	}
	_, refs := threadHeaders("<root@x>", prior)
	ids := strings.Fields(refs)
	if len(ids) != maxReferences+1 {
		t.Fatalf("References holds %d ids, want root + %d", len(ids), maxReferences)
	}
	if ids[0] != "<root@x>" || ids[len(ids)-1] != "<r24@x>" {
		t.Fatalf("References must keep the root and the newest replies: %v", ids)
	}
}

func TestNewMessageIDUsesSendingDomain(t *testing.T) {
	id := newMessageID("msg-uuid", "Acme Support <support@Acme.example>")
	if !regexp.MustCompile(`^<[0-9a-f-]{36}\.msg-uuid@acme\.example>$`).MatchString(id) {
		t.Fatalf("Message-ID = %q", id)
	}
	first, second := newMessageID("u", "support@acme.example"), newMessageID("u", "support@acme.example")
	if first == second {
		t.Fatal("Message-IDs must be unique per reply")
	}
}

func TestParseThreadAddress(t *testing.T) {
	token := strings.Repeat("a", 22) + "2345"
	cases := []struct {
		addr string
		ok   bool
	}{
		{"msg+" + token + "@reply.example.com", true},
		{"Ada <MSG+" + strings.ToUpper(token) + "@Reply.Example.com>", true},
		{"msg+" + token + "@other.example.com", false},                   // wrong domain
		{"msg+" + token[:20] + "@reply.example.com", false},              // wrong length
		{"msg+" + strings.Repeat("1", 26) + "@reply.example.com", false}, // not base32
		{"support@reply.example.com", false},
		{"msg+" + token + "@reply.example.com.evil.test", false},
		{"", false},
	}
	for _, tc := range cases {
		got, ok := ParseThreadAddress(tc.addr, "reply.example.com")
		if ok != tc.ok {
			t.Fatalf("ParseThreadAddress(%q) ok = %v, want %v", tc.addr, ok, tc.ok)
		}
		if ok && got != token {
			t.Fatalf("ParseThreadAddress(%q) token = %q", tc.addr, got)
		}
	}
}

func TestThreadAddressRoundTrip(t *testing.T) {
	s := &Service{cfg: Config{InboundDomain: "reply.example.com"}}
	msg := &models.Message{ThreadToken: newThreadToken()}
	token, ok := ParseThreadAddress(s.ThreadReplyTo(msg), "reply.example.com")
	if !ok || token != msg.ThreadToken {
		t.Fatalf("ThreadReplyTo output does not parse back: %q", s.ThreadReplyTo(msg))
	}
}

type fakeThreadStore struct {
	msgs    map[string]*models.Message
	replies []*models.MessageReply
	updates []map[string]any
	dupe    bool
}

func (f *fakeThreadStore) FindByThreadToken(token string) (*models.Message, error) {
	if m, ok := f.msgs[token]; ok {
		return m, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeThreadStore) AddInboundReply(r *models.MessageReply) (bool, error) {
	if f.dupe {
		return false, nil
	}
	f.replies = append(f.replies, r)
	return true, nil
}

func (f *fakeThreadStore) UpdateFields(_ uint, fields map[string]any) error {
	f.updates = append(f.updates, fields)
	return nil
}

type fakeReplyEnqueuer struct{ ids []uint }

func (f *fakeReplyEnqueuer) EnqueueMessageReply(id uint) error {
	f.ids = append(f.ids, id)
	return nil
}

// routerFixture builds a router whose one thread belongs to workspace 1.
func routerFixture() (*ThreadRouter, *fakeThreadStore, *fakeReplyEnqueuer, string) {
	wsOfMessage := uint(1)
	token := newThreadToken()
	store := &fakeThreadStore{msgs: map[string]*models.Message{
		token: {ID: 7, UUID: "m-7", WorkspaceID: &wsOfMessage, SenderEmail: "ada@example.com", State: models.MessageStateReplied},
	}}
	enq := &fakeReplyEnqueuer{}
	r := NewThreadRouter(store, "reply.example.com")
	r.SetEnqueuer(enq)
	return r, store, enq, "msg+" + token + "@reply.example.com"
}

func inboundFrom(ws uint, sender string) *models.InboundEmail {
	return &models.InboundEmail{ID: 42, UUID: "in-42", WorkspaceID: &ws, Sender: sender,
		Subject: "Re: hi", TextBody: "thanks", MessageID: "abc@mail.example"}
}

func TestRouteReplyIgnoresNonThreadRecipients(t *testing.T) {
	r, store, _, _ := routerFixture()
	routed, err := r.RouteReply(context.Background(), inboundFrom(1, "ada@example.com"), []string{"support@reply.example.com"})
	if err != nil || routed || len(store.replies) != 0 {
		t.Fatalf("routed = %v, err = %v, replies = %d", routed, err, len(store.replies))
	}
}

func TestRouteReplyIgnoresUnknownToken(t *testing.T) {
	r, store, _, _ := routerFixture()
	unknown := "msg+" + newThreadToken() + "@reply.example.com"
	routed, err := r.RouteReply(context.Background(), inboundFrom(1, "ada@example.com"), []string{unknown})
	if err != nil || routed || len(store.replies) != 0 {
		t.Fatalf("routed = %v, err = %v", routed, err)
	}
}

func TestRouteReplyRefusesOtherWorkspace(t *testing.T) {
	r, store, enq, addr := routerFixture()
	routed, err := r.RouteReply(context.Background(), inboundFrom(2, "ada@example.com"), []string{addr})
	if err != nil || routed {
		t.Fatalf("routed = %v, err = %v; a token must not cross workspaces", routed, err)
	}
	if len(store.replies) != 0 || len(store.updates) != 0 || len(enq.ids) != 0 {
		t.Fatal("a cross-workspace reply must have no side effects")
	}
}

func TestRouteReplyAttachesAndReopens(t *testing.T) {
	r, store, enq, addr := routerFixture()
	bus := realtime.NewMemory()
	r.SetRealtime(bus)
	ch, unsub := bus.Subscribe(context.Background(), realtime.WorkspaceTopic(1))
	defer unsub()

	routed, err := r.RouteReply(context.Background(), inboundFrom(1, "Ada <ADA@example.com>"), []string{"x@y.test", addr})
	if err != nil || !routed {
		t.Fatalf("routed = %v, err = %v", routed, err)
	}
	if len(store.replies) != 1 {
		t.Fatalf("replies = %d", len(store.replies))
	}
	reply := store.replies[0]
	if reply.Kind != models.MessageReplyKindInbound || reply.MessageID != 7 || *reply.InboundEmailID != 42 {
		t.Fatalf("reply = %+v", reply)
	}
	if reply.MessageIDHeader != "<abc@mail.example>" {
		t.Fatalf("MessageIDHeader = %q", reply.MessageIDHeader)
	}
	if reply.SenderMismatch {
		t.Fatal("the original sender, in any case or display form, is not a mismatch")
	}
	if got := store.updates[0]["state"]; got != models.MessageStateOpen {
		t.Fatalf("state update = %v, want open", got)
	}
	if v, ok := store.updates[0]["read_at"]; !ok || v != nil {
		t.Fatal("a reply must mark the thread unread")
	}
	if len(enq.ids) != 1 {
		t.Fatal("the reply follow-up task was not enqueued")
	}
	select {
	case ev := <-ch:
		if ev.Type != EventReplyReceived {
			t.Fatalf("event = %q", ev.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("no realtime event")
	}
}

func TestRouteReplyFlagsSenderMismatch(t *testing.T) {
	r, store, _, addr := routerFixture()
	if routed, _ := r.RouteReply(context.Background(), inboundFrom(1, "mallory@evil.test"), []string{addr}); !routed {
		t.Fatal("a mismatched sender is still attached, flagged")
	}
	if !store.replies[0].SenderMismatch {
		t.Fatal("expected SenderMismatch")
	}
}

func TestRouteReplyDuplicateIsDoneWithoutSideEffects(t *testing.T) {
	r, store, enq, addr := routerFixture()
	store.dupe = true
	routed, err := r.RouteReply(context.Background(), inboundFrom(1, "ada@example.com"), []string{addr})
	if err != nil || !routed {
		t.Fatalf("a retried delivery must still count as routed: %v %v", routed, err)
	}
	if len(store.updates) != 0 || len(enq.ids) != 0 {
		t.Fatal("a duplicate must not reopen the thread or re-enqueue")
	}
}

func TestRouteReplyKeepsSpamThreadState(t *testing.T) {
	r, store, _, addr := routerFixture()
	for _, m := range store.msgs {
		m.State = models.MessageStateSpam
	}
	if routed, _ := r.RouteReply(context.Background(), inboundFrom(1, "ada@example.com"), []string{addr}); !routed {
		t.Fatal("expected routed")
	}
	if _, ok := store.updates[0]["state"]; ok {
		t.Fatal("a reply must not pull a thread out of spam")
	}
}
