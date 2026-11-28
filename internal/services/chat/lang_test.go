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
	"strings"
	"testing"

	"github.com/goposta/posta/internal/models"
)

func TestConversationLanguage(t *testing.T) {
	cases := []struct{ widget, visitor, want string }{
		{"auto", "fr-FR", "fr"},
		{"auto", "pt-BR", "pt"},
		{"auto", "sw", "sw"},
		{"auto", "ja", "en"},
		{"auto", "", "en"},
		{"de", "fr", "de"},
		{"ln", "", "ln"},
	}
	for _, c := range cases {
		if got := ConversationLanguage(c.widget, c.visitor); got != c.want {
			t.Errorf("ConversationLanguage(%q, %q) = %q, want %q", c.widget, c.visitor, got, c.want)
		}
	}
}

func TestValidWidgetLanguage(t *testing.T) {
	for _, ok := range []string{"auto", "en", "fr", "es", "pt", "de", "sw", "ln"} {
		if !ValidWidgetLanguage(ok) {
			t.Errorf("%q refused", ok)
		}
	}
	for _, bad := range []string{"", "xx", "english"} {
		if ValidWidgetLanguage(bad) {
			t.Errorf("%q accepted", bad)
		}
	}
}

// Every language the widget offers must have the follow-up email's words, or the visitor gets English.
func TestEmailWordsCoverEveryLanguage(t *testing.T) {
	for _, l := range Languages {
		w, ok := emailText[l]
		if !ok || w.Support == "" || w.ReplyHere == "" || w.ContinueLead == "" || w.Or == "" || w.ContinueInChat == "" || !strings.Contains(w.Subject, "%s") {
			t.Errorf("email words for %q incomplete: %+v", l, w)
		}
	}
}

func TestOfflineEmailSpeaksTheConversationLanguage(t *testing.T) {
	s := &Service{}
	conv := &models.ChatConversation{UUID: "c-2", StartPageURL: "https://acme.test/", Language: "fr", Visitor: &models.ChatVisitor{Email: "marie@example.com"}}
	w := &models.ChatWidget{Title: "Acme", ReplyFrom: "support@acme.test"}
	req := s.offlineEmail(conv, w, []models.ChatMessage{{Body: "Bonjour"}})
	if req.Subject != "Nouvelle réponse de Acme" || !strings.Contains(req.Text, "Poursuivez la conversation dans le chat") || !strings.Contains(req.Text, "Support:") {
		t.Fatalf("French email = %q / %q", req.Subject, req.Text)
	}
}
