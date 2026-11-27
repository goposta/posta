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
)

// Regression for GHSA-8cvm-4f8p-xgf8: the bounce webhook's lookup must not return another workspace's subscriber.
func TestFindAllByEmail_ScopedToWorkspace(t *testing.T) {
	db := testDB(t)
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })
	if err := tx.AutoMigrate(&models.Subscriber{}); err != nil {
		t.Skipf("skipping: cannot migrate subscribers: %v", err)
	}

	userA := createUser(t, tx, "bounce-scope-a@test.local")
	userB := createUser(t, tx, "bounce-scope-b@test.local")
	wsA, wsB := uintPtr(1001), uintPtr(1002)
	for _, s := range []models.Subscriber{
		{UserID: userA, WorkspaceID: wsA, Email: "shared@example.com", Status: models.SubscriberStatusSubscribed},
		{UserID: userB, WorkspaceID: wsB, Email: "shared@example.com", Status: models.SubscriberStatusSubscribed},
	} {
		if err := tx.Create(&s).Error; err != nil {
			t.Fatalf("create subscriber: %v", err)
		}
	}

	repo := NewSubscriberRepository(tx)
	var got []models.Subscriber
	if err := repo.FindAllByEmail(ResourceScope{UserID: userA, WorkspaceID: wsA}, "shared@example.com", &got); err != nil {
		t.Fatalf("FindAllByEmail: %v", err)
	}
	if len(got) != 1 || got[0].WorkspaceID == nil || *got[0].WorkspaceID != *wsA {
		t.Fatalf("FindAllByEmail returned %+v, want only workspace %d's row", got, *wsA)
	}

	got = nil
	if err := repo.FindAllByEmail(ResourceScope{UserID: userA}, "shared@example.com", &got); err != nil {
		t.Fatalf("FindAllByEmail without workspace: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("FindAllByEmail without a workspace returned %d rows, want 0", len(got))
	}
}
