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

func seedActor(t *testing.T, tx *gorm.DB, email string) uint {
	t.Helper()
	u := &models.User{Name: "Jonas Kaninda", Email: email, PasswordHash: "x"}
	if err := tx.Create(u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u.ID
}

func seedWorkspace(t *testing.T, tx *gorm.DB, ownerID uint, slug string) uint {
	t.Helper()
	ws := &models.Workspace{Name: "Acme", Slug: slug, OwnerID: ownerID}
	if err := tx.Create(ws).Error; err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	return ws.ID
}

func TestFormUpdateDoesNotCascadeIntoUsers(t *testing.T) {
	db := testDB(t)
	if err := db.AutoMigrate(&models.User{}, &models.Workspace{}, &models.Form{}); err != nil {
		t.Skipf("skipping: cannot migrate schema: %v", err)
	}
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })

	userID := seedActor(t, tx, "form-cascade@test.local")
	wsID := seedWorkspace(t, tx, userID, "form-cascade")

	repo := NewFormRepository(tx)
	form := &models.Form{
		WorkspaceID: &wsID, Name: "Contact", Slug: "contact",
		PublicKey: "form-cascade-key", Status: models.FormStatusActive,
		LastEditedByID: &userID,
	}
	if err := repo.Create(form); err != nil {
		t.Fatalf("create form: %v", err)
	}

	// Read it back the way the handler does — with LastEditedBy preloaded.
	loaded, err := repo.FindByID(form.ID)
	if err != nil {
		t.Fatalf("find form: %v", err)
	}
	if loaded.LastEditedBy == nil {
		t.Fatal("expected LastEditedBy to be preloaded; this test proves nothing without it")
	}

	loaded.Name = "Contact form"
	if err := repo.Update(loaded); err != nil {
		t.Fatalf("update form must not cascade into users: %v", err)
	}
}

func TestMessageUpdateDoesNotCascadeIntoUsers(t *testing.T) {
	db := testDB(t)
	if err := db.AutoMigrate(
		&models.User{}, &models.Workspace{}, &models.Form{},
		&models.Message{}, &models.MessageReply{},
	); err != nil {
		t.Skipf("skipping: cannot migrate schema: %v", err)
	}
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })

	userID := seedActor(t, tx, "msg-cascade@test.local")
	wsID := seedWorkspace(t, tx, userID, "msg-cascade")

	formRepo := NewFormRepository(tx)
	form := &models.Form{
		WorkspaceID: &wsID, Name: "Contact", Slug: "contact",
		PublicKey: "msg-cascade-key", Status: models.FormStatusActive,
	}
	if err := formRepo.Create(form); err != nil {
		t.Fatalf("create form: %v", err)
	}

	repo := NewMessageRepository(tx)
	msg := &models.Message{
		WorkspaceID: &wsID, FormID: form.ID,
		SenderEmail: "ada@example.com", Subject: "Hello",
		Status: models.MessageStatusReceived, State: models.MessageStateNew,
		ThreadToken: "msg-cascade-token", AssignedToID: &userID,
	}
	if err := repo.Create(msg); err != nil {
		t.Fatalf("create message: %v", err)
	}
	if err := repo.AddReply(&models.MessageReply{
		MessageID: msg.ID, WorkspaceID: &wsID, Kind: models.MessageReplyKindOperator,
		AuthorID: &userID, FromAddr: "support@example.com", ToAddr: "ada@example.com",
	}); err != nil {
		t.Fatalf("add reply: %v", err)
	}

	scope := ResourceScope{UserID: userID, WorkspaceID: &wsID}
	loaded, err := repo.FindByUUIDForScope(scope, msg.UUID)
	if err != nil {
		t.Fatalf("find message: %v", err)
	}
	if loaded.AssignedTo == nil || len(loaded.Replies) == 0 || loaded.Replies[0].Author == nil {
		t.Fatal("expected AssignedTo and Replies[].Author to be preloaded; this test proves nothing without them")
	}

	loaded.State = models.MessageStateReplied
	if err := repo.Update(loaded); err != nil {
		t.Fatalf("update message must not cascade into users: %v", err)
	}
}
