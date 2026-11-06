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
	"encoding/json"
	"strings"
	"testing"

	"github.com/goposta/posta/internal/models"
)

func TestVisitorTokenRoundTrip(t *testing.T) {
	token, hash, err := NewVisitorToken()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(token, VisitorTokenPrefix) {
		t.Fatalf("token %q lacks the %s prefix", token, VisitorTokenPrefix)
	}
	if strings.Contains(hash, token) || hash == token {
		t.Fatal("the stored hash contains the token")
	}
	if HashVisitorToken(token) != hash {
		t.Fatal("hashing the token again gives a different hash")
	}
	other, _, _ := NewVisitorToken()
	if HashVisitorToken(other) == hash {
		t.Fatal("two tokens share a hash")
	}
}

func TestBearerVisitorToken(t *testing.T) {
	token, _, _ := NewVisitorToken()
	cases := map[string]string{
		"Bearer " + token:                        token,
		"bearer " + token:                        token,
		"Bearer  " + token + " ":                 token,
		"Bearer psk_live_abc":                    "",
		"Basic " + token:                         "",
		token:                                    "",
		"Bearer ":                                "",
		"":                                       "",
		"Bearer pcv_" + strings.Repeat("a", 200): "",
	}
	for header, want := range cases {
		if got := BearerVisitorToken(header); got != want {
			t.Errorf("BearerVisitorToken(%q) = %q, want %q", header, got, want)
		}
	}
}

func TestCleanBody(t *testing.T) {
	cases := map[string]string{
		"  hello  ":                    "hello",
		"line one\r\nline two":         "line one\nline two",
		"tab\there":                    "tab\there",
		"nul\x00byte":                  "nulbyte",
		"bell\x07 and esc\x1b[31m red": "bell and esc[31m red",
		"café":                        "café", // NFC
		"<script>alert(1)</script>":    "<script>alert(1)</script>",
		"\x00\x01 ":                    "",
	}
	for in, want := range cases {
		if got := CleanBody(in); got != want {
			t.Errorf("CleanBody(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCleanLine(t *testing.T) {
	if got := CleanLine("  Jane \n\t Doe ", 120); got != "Jane Doe" {
		t.Fatalf("CleanLine = %q", got)
	}
	if got := CleanLine("ééééé", 3); got != "ééé" {
		t.Fatalf("CleanLine cut runes wrong: %q", got)
	}
}

func TestValidAccentColor(t *testing.T) {
	for _, ok := range []string{"", "#7c3aed", "#ABCDEF"} {
		if !ValidAccentColor(ok) {
			t.Errorf("%q rejected", ok)
		}
	}
	for _, bad := range []string{"red", "#fff", "#7c3aed;}", "#7c3aedff", "url(x)", "#7c3aed\n"} {
		if ValidAccentColor(bad) {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestValidAvatarURL(t *testing.T) {
	for _, ok := range []string{"", "https://cdn.example.com/a.png"} {
		if !ValidAvatarURL(ok) {
			t.Errorf("%q rejected", ok)
		}
	}
	for _, bad := range []string{"http://example.com/a.png", "javascript:alert(1)", "data:image/png;base64,AA", "//example.com/a.png", "https://"} {
		if ValidAvatarURL(bad) {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestCleanPageURL(t *testing.T) {
	if got := CleanPageURL(" https://example.com/pricing?x=1 "); got != "https://example.com/pricing?x=1" {
		t.Fatalf("CleanPageURL = %q", got)
	}
	for _, bad := range []string{"javascript:alert(1)", "ftp://example.com", "not a url", strings.Repeat("https://e.com/", 200)} {
		if got := CleanPageURL(bad); got != "" {
			t.Errorf("CleanPageURL(%q) = %q, want empty", bad, got)
		}
	}
}

func TestCleanClientID(t *testing.T) {
	if got := cleanClientID("6f1c2a9e-3b7d-4e1a-9c0b-1f2e3d4c5b6a"); got == "" {
		t.Fatal("a UUID was dropped")
	}
	for _, bad := range []string{"has space", "'; DROP", strings.Repeat("a", 37)} {
		if got := cleanClientID(bad); got != "" {
			t.Errorf("cleanClientID(%q) = %q, want empty", bad, got)
		}
	}
}

func TestCheckBody(t *testing.T) {
	if _, err := checkBody(" \n\x00 ", 100); err != ErrEmptyBody {
		t.Fatalf("blank body: err = %v", err)
	}
	if _, err := checkBody(strings.Repeat("a", 101), 100); err != ErrBodyTooLong {
		t.Fatalf("long body: err = %v", err)
	}
	if body, err := checkBody(" hi ", 100); err != nil || body != "hi" {
		t.Fatalf("checkBody = %q, %v", body, err)
	}
}

func TestAcquireStreamCapsStreams(t *testing.T) {
	s := &Service{cfg: Config{MaxStreams: 2}}
	if !s.AcquireStream() || !s.AcquireStream() {
		t.Fatal("streams under the cap were refused")
	}
	if s.AcquireStream() {
		t.Fatal("a stream over the cap was allowed")
	}
	s.ReleaseStream()
	if !s.AcquireStream() {
		t.Fatal("a released slot was not reusable")
	}
}

func TestToVisitorMessageHidesAgentIdentity(t *testing.T) {
	agentID := uint(42)
	m := &models.ChatMessage{
		UUID: "m1", Seq: 3, AuthorKind: models.ChatAuthorAgent, AuthorID: &agentID,
		Body: "hello", Author: &models.ActorRef{ID: agentID, Name: "Jane"},
	}
	shown := ToVisitorMessage(m, true)
	if shown.AgentName != "Jane" || shown.Author != models.ChatAuthorAgent || shown.Seq != 3 {
		t.Fatalf("ToVisitorMessage = %+v", shown)
	}
	encoded, _ := json.Marshal(shown)
	if strings.Contains(string(encoded), "42") || strings.Contains(string(encoded), "author_id") {
		t.Fatalf("the agent's user ID reached the visitor: %s", encoded)
	}
	if hidden := ToVisitorMessage(m, false); hidden.AgentName != "" {
		t.Fatalf("agent name shown with show_agent_names off: %+v", hidden)
	}
}

func TestVisitorNeverSeesSpamState(t *testing.T) {
	c := &models.ChatConversation{UUID: "c1", State: models.ChatStateSpam}
	if got := ToVisitorConversation(c).State; got != string(models.ChatStateClosed) {
		t.Fatalf("state = %q, want closed", got)
	}
}
