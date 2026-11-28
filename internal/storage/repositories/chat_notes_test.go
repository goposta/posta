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

package repositories

import (
	"testing"

	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
)

func chatFixture(t *testing.T) (*gorm.DB, *ChatRepository, *models.ChatConversation) {
	t.Helper()
	tx := testDB(t).Begin()
	t.Cleanup(func() { tx.Rollback() })
	if err := tx.AutoMigrate(&models.ChatWidget{}, &models.ChatVisitor{}, &models.ChatConversation{}, &models.ChatMessage{}); err != nil {
		t.Skipf("skipping: cannot migrate chat tables: %v", err)
	}
	ws := uintPtr(4242)
	repo := NewChatRepository(tx)
	w := &models.ChatWidget{WorkspaceID: ws, Name: "notes-test", DisplayName: "Notes test", PublicKey: "pcw_notes_test_key", Status: models.ChatWidgetStatusActive}
	if err := repo.CreateWidget(w); err != nil {
		t.Fatalf("create widget: %v", err)
	}
	v := &models.ChatVisitor{WorkspaceID: ws, WidgetID: w.ID}
	if err := tx.Create(v).Error; err != nil {
		t.Fatalf("create visitor: %v", err)
	}
	conv := &models.ChatConversation{WorkspaceID: ws, WidgetID: w.ID, VisitorID: v.ID, State: models.ChatStateOpen}
	if err := tx.Create(conv).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	return tx, repo, conv
}

func appendMsg(t *testing.T, repo *ChatRepository, conv *models.ChatConversation, kind, body string) *models.ChatMessage {
	t.Helper()
	m := &models.ChatMessage{WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID, AuthorKind: kind, Body: body}
	if _, err := repo.AppendMessage(m); err != nil {
		t.Fatalf("append %s: %v", kind, err)
	}
	return m
}

// A note must not make a waiting conversation look answered, nor show its text in the visitor's preview.
func TestAppendNoteOnlyTakesASeq(t *testing.T) {
	tx, repo, conv := chatFixture(t)
	appendMsg(t, repo, conv, models.ChatAuthorVisitor, "Hello, my invoice is wrong")
	note := appendMsg(t, repo, conv, models.ChatAuthorNote, "Customer on the legacy plan")

	var got models.ChatConversation
	if err := tx.First(&got, conv.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.LastSeq != note.Seq || note.Seq != 2 {
		t.Fatalf("last_seq = %d, note seq = %d; want both 2", got.LastSeq, note.Seq)
	}
	if got.LastAuthorKind != models.ChatAuthorVisitor || got.LastMessagePreview != "Hello, my invoice is wrong" {
		t.Fatalf("last author %q, preview %q: the note leaked into what the visitor sees", got.LastAuthorKind, got.LastMessagePreview)
	}
	if got.FirstResponseAt != nil || !got.AwaitingReply() {
		t.Fatalf("a note counted as a reply: first_response_at=%v awaiting=%v", got.FirstResponseAt, got.AwaitingReply())
	}
}

func TestVisitorReadsSkipNotes(t *testing.T) {
	_, repo, conv := chatFixture(t)
	appendMsg(t, repo, conv, models.ChatAuthorVisitor, "Hi")
	appendMsg(t, repo, conv, models.ChatAuthorNote, "internal")
	appendMsg(t, repo, conv, models.ChatAuthorAgent, "Hello!")

	visitor, err := repo.MessagesAfter(conv.ID, 0, 50, false)
	if err != nil {
		t.Fatal(err)
	}
	agent, err := repo.MessagesAfter(conv.ID, 0, 50, true)
	if err != nil {
		t.Fatal(err)
	}
	before, err := repo.MessagesBefore(conv.ID, 10, 50, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(visitor) != 2 || len(before) != 2 || len(agent) != 3 {
		t.Fatalf("visitor sees %d (before: %d), agent sees %d; want 2, 2 and 3", len(visitor), len(before), len(agent))
	}
	for _, m := range append(visitor, before...) {
		if m.AuthorKind == models.ChatAuthorNote {
			t.Fatal("a visitor read returned an internal note")
		}
	}
}

func TestCreateWidgetKeepsFalseDefaults(t *testing.T) {
	tx := testDB(t).Begin()
	t.Cleanup(func() { tx.Rollback() })
	if err := tx.AutoMigrate(&models.ChatWidget{}); err != nil {
		t.Skipf("skipping: cannot migrate chat widgets: %v", err)
	}
	w := &models.ChatWidget{WorkspaceID: uintPtr(4243), Name: "falsy", DisplayName: "Falsy", PublicKey: "pcw_falsy_test_key",
		Status: models.ChatWidgetStatusActive, StrictOrigin: false, ShowAgentNames: false, AskRating: false, OneOpenConversation: false, ScanEnabled: false}
	if err := NewChatRepository(tx).CreateWidget(w); err != nil {
		t.Fatal(err)
	}
	var got models.ChatWidget
	if err := tx.First(&got, w.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.StrictOrigin || got.ShowAgentNames || got.AskRating || got.OneOpenConversation || got.ScanEnabled {
		t.Fatalf("false settings came back true: %+v", got)
	}
}
