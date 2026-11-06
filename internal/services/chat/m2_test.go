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

package chat

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/goposta/posta/internal/models"
)

func TestShouldEmail(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	gone := OfflineFacts{
		VisitorLastSeen: now.Add(-3 * time.Minute), VisitorEmail: "jane@example.com",
		ReplyFrom: "support@acme.test", Status: models.MessageStatusReceived, State: models.ChatStateOpen, Now: now,
	}
	cases := []struct {
		name string
		edit func(f *OfflineFacts)
		want bool
	}{
		{"visitor left, all set", func(*OfflineFacts) {}, true},
		{"widget still open", func(f *OfflineFacts) { f.VisitorLive = true }, false},
		{"seen a minute ago", func(f *OfflineFacts) { f.VisitorLastSeen = now.Add(-time.Minute) }, false},
		{"exactly two minutes", func(f *OfflineFacts) { f.VisitorLastSeen = now.Add(-VisitorAwayAfter) }, true},
		{"no email given", func(f *OfflineFacts) { f.VisitorEmail = "" }, false},
		{"no reply sender", func(f *OfflineFacts) { f.ReplyFrom = "" }, false},
		{"quarantined", func(f *OfflineFacts) { f.Status = models.MessageStatusQuarantined }, false},
		{"spam", func(f *OfflineFacts) { f.State = models.ChatStateSpam }, false},
		{"flagged still gets it", func(f *OfflineFacts) { f.Status = models.MessageStatusFlagged }, true},
		{"closed still gets it", func(f *OfflineFacts) { f.State = models.ChatStateClosed }, true},
	}
	for _, tc := range cases {
		f := gone
		tc.edit(&f)
		if got := ShouldEmail(f); got != tc.want {
			t.Errorf("%s: ShouldEmail = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestStripQuotedReply(t *testing.T) {
	cases := map[string]string{
		"Thanks, that works!\n\nOn Tue, Nov 4, 2026 at 10:00 AM Acme <support@acme.test> wrote:\n> Yes we ship to Canada.": "Thanks, that works!",
		"Merci !\n\nLe mar. 4 nov. 2026 à 10:00, Acme <support@acme.test> a écrit :\n> Oui.":                               "Merci !",
		"Sounds good.\r\n\r\n-----Original Message-----\r\nFrom: Acme":                                                     "Sounds good.",
		"One more thing\n> quoted\n> lines\n":                                                                              "One more thing",
		"> only a quote":                                                                                                   "",
		"Line one\n> inline quote\nLine two":                                                                               "Line one\n> inline quote\nLine two",
	}
	for in, want := range cases {
		if got := StripQuotedReply(in); got != want {
			t.Errorf("StripQuotedReply(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseChatThreadAddress(t *testing.T) {
	token := strings.Repeat("a", 26)
	cases := map[string]bool{
		"chat+" + token + "@reply.acme.test":        true,
		"Acme <CHAT+" + token + "@Reply.Acme.Test>": true,
		"msg+" + token + "@reply.acme.test":         false,
		"chat+" + token + "@other.test":             false,
		"chat+" + token[:25] + "@reply.acme.test":   false,
		"chat+" + token[:25] + "1@reply.acme.test":  false,
	}
	for addr, want := range cases {
		if _, ok := ParseChatThreadAddress(addr, "reply.acme.test"); ok != want {
			t.Errorf("ParseChatThreadAddress(%q) = %v, want %v", addr, ok, want)
		}
	}
	if got := newThreadToken(); got == nil {
		t.Fatal("no thread token")
	} else if _, ok := ParseChatThreadAddress("chat+"+*got+"@reply.acme.test", "reply.acme.test"); !ok {
		t.Fatalf("a generated token %q does not parse", *got)
	}
}

func TestOfflineEmailEscapesAndThreads(t *testing.T) {
	token := strings.Repeat("b", 26)
	s := &Service{cfg: Config{InboundDomain: "reply.acme.test"}}
	conv := &models.ChatConversation{
		UUID: "c-1", ThreadToken: &token, StartPageURL: "https://acme.test/pricing#top",
		Visitor: &models.ChatVisitor{Email: "jane@example.com"},
	}
	w := &models.ChatWidget{Title: "Acme support", ReplyFrom: "support@acme.test", ReplyFromName: "Acme\r\nBcc: x@evil.test", ShowAgentNames: true}
	req := s.offlineEmail(conv, w, []models.ChatMessage{
		{Body: "<script>alert(1)</script> see https://acme.test", Author: &models.ActorRef{Name: "Bob <b>"}},
	})
	if strings.Contains(req.HTML, "<script>") || strings.Contains(req.HTML, "Bob <b>") {
		t.Fatalf("message or name not escaped: %s", req.HTML)
	}
	if strings.Contains(req.From, "\n") || strings.Contains(req.From, "\r") {
		t.Fatalf("header injection in From: %q", req.From)
	}
	if req.SystemHeaders["Reply-To"] != "chat+"+token+"@reply.acme.test" {
		t.Fatalf("Reply-To = %q", req.SystemHeaders["Reply-To"])
	}
	if !strings.Contains(req.Text, "https://acme.test/pricing#posta-chat=c-1") {
		t.Fatalf("continue link missing or wrong: %s", req.Text)
	}
	if len(req.To) != 1 || req.To[0] != "jane@example.com" || req.Subject != "New reply from Acme support" {
		t.Fatalf("addressing = %v %q", req.To, req.Subject)
	}

	// Without an inbound domain there is no Reply-To, so the email says to come back to the chat instead.
	s.cfg.InboundDomain = ""
	req = s.offlineEmail(conv, w, []models.ChatMessage{{Body: "hi"}})
	if req.SystemHeaders != nil || !strings.Contains(req.Text, "Continue the conversation in the chat") {
		t.Fatalf("no-inbound email = %+v", req)
	}
}

func TestBusinessHours(t *testing.T) {
	h := models.BusinessHours{Timezone: "Europe/Paris", Days: []models.HoursRange{{Day: 3, From: "09:00", To: "17:30"}}}
	if err := h.Validate(); err != nil {
		t.Fatal(err)
	}
	// Wednesday 2026-11-04, Paris is UTC+1.
	open := time.Date(2026, 11, 4, 8, 0, 0, 0, time.UTC)      // 09:00 Paris
	closing := time.Date(2026, 11, 4, 16, 30, 0, 0, time.UTC) // 17:30 Paris
	if !h.Open(open) || h.Open(closing) || h.Open(open.Add(-time.Minute)) {
		t.Fatal("range edges wrong: from is inclusive, to exclusive")
	}
	if h.Open(open.Add(24 * time.Hour)) {
		t.Fatal("open on a Thursday")
	}
	if !(models.BusinessHours{}).Open(open) {
		t.Fatal("no hours must mean always open")
	}
	for _, bad := range []models.BusinessHours{
		{Timezone: "Mars/Olympus"},
		{Days: []models.HoursRange{{Day: 7, From: "09:00", To: "10:00"}}},
		{Days: []models.HoursRange{{Day: 1, From: "10:00", To: "09:00"}}},
		{Days: []models.HoursRange{{Day: 1, From: "9:00", To: "10:00"}}},
	} {
		if bad.Validate() == nil {
			t.Errorf("accepted %+v", bad)
		}
	}
}

func TestFreshPresence(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	got := freshPresence(map[string]string{
		"1": "online:" + itoa(now.Unix()-10),
		"2": "away:" + itoa(now.Unix()-70),
		"3": "online:" + itoa(now.Unix()-80), // missed its heartbeat
		"x": "online:1",
		"4": "garbage",
	}, now)
	if len(got) != 2 || got[1] != StatusOnline || got[2] != StatusAway {
		t.Fatalf("freshPresence = %v", got)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
