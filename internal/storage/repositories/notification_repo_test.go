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

func notificationDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := testDB(t)
	if err := db.AutoMigrate(&models.Notification{}); err != nil {
		t.Skipf("skipping: cannot migrate notifications: %v", err)
	}
	return db
}

func alert(userID uint, wsID *uint, fingerprint, title string) *models.Notification {
	return &models.Notification{
		UserID:      userID,
		WorkspaceID: wsID,
		Kind:        models.NotificationKindAlert,
		Category:    models.NotificationCategoryDomains,
		Severity:    models.NotificationWarning,
		Title:       title,
		DedupKey:    "domains:unverified",
		Fingerprint: fingerprint,
	}
}

func mustUpsert(t *testing.T, r *NotificationRepository, n *models.Notification) *models.Notification {
	t.Helper()
	if err := r.Upsert(n); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	return n
}

func reload(t *testing.T, tx *gorm.DB, id uint) models.Notification {
	t.Helper()
	var got models.Notification
	if err := tx.First(&got, id).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	return got
}

// A condition that persists unchanged must fold into its row, not add one on every dashboard load.
func TestUpsertFoldsRepeatedCondition(t *testing.T) {
	db := notificationDB(t)
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })

	userID := createUser(t, tx, "inbox-fold@test.local")
	ws := uintPtr(4001)
	repo := &NotificationRepository{db: tx}

	first := mustUpsert(t, repo, alert(userID, ws, "n=3", "3 domains not verified"))
	second := mustUpsert(t, repo, alert(userID, ws, "n=3", "3 domains not verified"))

	if first.ID != second.ID {
		t.Fatalf("expected the same row, got %d then %d", first.ID, second.ID)
	}

	var count int64
	tx.Model(&models.Notification{}).Where("user_id = ?", userID).Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 notification, got %d", count)
	}
}

// An unchanged condition stays dismissed, which stops the banner reappearing on every page load.
func TestDismissSurvivesUnchangedCondition(t *testing.T) {
	db := notificationDB(t)
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })

	userID := createUser(t, tx, "inbox-stay@test.local")
	ws := uintPtr(4002)
	repo := &NotificationRepository{db: tx}

	n := mustUpsert(t, repo, alert(userID, ws, "n=3", "3 domains not verified"))
	if err := repo.Dismiss(userID, []uint{n.ID}); err != nil {
		t.Fatalf("dismiss: %v", err)
	}

	mustUpsert(t, repo, alert(userID, ws, "n=3", "3 domains not verified"))

	got := reload(t, tx, n.ID)
	if got.DismissedAt == nil {
		t.Fatal("an unchanged condition must stay dismissed")
	}
	if got.ReadAt == nil {
		t.Fatal("dismissing should also mark the item read")
	}
}

// A materially worse condition re-arms the item, so one dismissal does not silence it forever.
func TestDismissResurfacesOnChangedCondition(t *testing.T) {
	db := notificationDB(t)
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })

	userID := createUser(t, tx, "inbox-resurface@test.local")
	ws := uintPtr(4003)
	repo := &NotificationRepository{db: tx}

	n := mustUpsert(t, repo, alert(userID, ws, "n=3", "3 domains not verified"))
	if err := repo.Dismiss(userID, []uint{n.ID}); err != nil {
		t.Fatalf("dismiss: %v", err)
	}

	mustUpsert(t, repo, alert(userID, ws, "n=4", "4 domains not verified"))

	got := reload(t, tx, n.ID)
	if got.DismissedAt != nil {
		t.Fatal("a worsened condition must resurface")
	}
	if got.ReadAt != nil {
		t.Fatal("a resurfaced item must be unread again")
	}
	if got.Title != "4 domains not verified" {
		t.Fatalf("expected the title to update in place, got %q", got.Title)
	}
}

// A cleared condition is resolved, leaves the banner, and stays in the inbox as history.
func TestResolveExceptClosesClearedConditions(t *testing.T) {
	db := notificationDB(t)
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })

	userID := createUser(t, tx, "inbox-resolve@test.local")
	ws := uintPtr(4004)
	repo := &NotificationRepository{db: tx}

	n := mustUpsert(t, repo, alert(userID, ws, "n=3", "3 domains not verified"))

	if err := repo.ResolveExcept(userID, ws, nil); err != nil {
		t.Fatalf("resolve: %v", err)
	}

	got := reload(t, tx, n.ID)
	if got.ResolvedAt == nil {
		t.Fatal("a cleared condition must be resolved")
	}

	open, err := repo.Open(userID, ws)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if len(open) != 0 {
		t.Fatalf("a resolved item must leave the banner, got %d", len(open))
	}

	// Resolved, then firing again: a new row, because the old one is history.
	again := mustUpsert(t, repo, alert(userID, ws, "n=1", "1 domain not verified"))
	if again.ID == n.ID {
		t.Fatal("a condition that fires after resolving should open a new item")
	}
}

// Platform announcements have no workspace and must reach the user in every workspace's banner.
func TestOpenIncludesPlatformItemsAndExcludesOtherWorkspaces(t *testing.T) {
	db := notificationDB(t)
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })

	userID := createUser(t, tx, "inbox-scope@test.local")
	here, elsewhere := uintPtr(4005), uintPtr(4006)
	repo := &NotificationRepository{db: tx}

	mustUpsert(t, repo, alert(userID, here, "n=1", "here"))
	mustUpsert(t, repo, alert(userID, elsewhere, "n=1", "elsewhere"))
	mustUpsert(t, repo, &models.Notification{
		UserID:      userID,
		Kind:        models.NotificationKindAnnouncement,
		Category:    models.NotificationCategoryPlatform,
		Severity:    models.NotificationInfo,
		Title:       "platform",
		DedupKey:    "announcement:1",
		Fingerprint: "announcement=1",
	})

	open, err := repo.Open(userID, here)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	seen := map[string]bool{}
	for _, n := range open {
		seen[n.Title] = true
	}
	if !seen["here"] || !seen["platform"] {
		t.Fatalf("expected the workspace item and the platform item, got %v", seen)
	}
	if seen["elsewhere"] {
		t.Fatal("another workspace's alert must not appear on this banner")
	}
}

// One user's inbox is not another's, including for mark-read and dismiss.
func TestInboxIsScopedToTheUser(t *testing.T) {
	db := notificationDB(t)
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })

	mine := createUser(t, tx, "inbox-mine@test.local")
	theirs := createUser(t, tx, "inbox-theirs@test.local")
	ws := uintPtr(4007)
	repo := &NotificationRepository{db: tx}

	n := mustUpsert(t, repo, alert(theirs, ws, "n=1", "not yours"))

	if err := repo.Dismiss(mine, []uint{n.ID}); err != nil {
		t.Fatalf("dismiss: %v", err)
	}
	if reload(t, tx, n.ID).DismissedAt != nil {
		t.Fatal("a user must not be able to dismiss another user's notification")
	}

	if err := repo.MarkRead(mine, []uint{n.ID}); err != nil {
		t.Fatalf("mark read: %v", err)
	}
	if reload(t, tx, n.ID).ReadAt != nil {
		t.Fatal("a user must not be able to read another user's notification")
	}
}

// Pruning keeps live conditions whatever their age.
func TestPruneKeepsOpenItems(t *testing.T) {
	db := notificationDB(t)
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })

	userID := createUser(t, tx, "inbox-prune@test.local")
	ws := uintPtr(4008)
	repo := &NotificationRepository{db: tx}

	open := mustUpsert(t, repo, alert(userID, ws, "n=1", "still true"))
	settled := mustUpsert(t, repo, &models.Notification{
		UserID:      userID,
		WorkspaceID: ws,
		Kind:        models.NotificationKindAlert,
		Category:    models.NotificationCategorySecurity,
		Severity:    models.NotificationWarning,
		Title:       "settled",
		DedupKey:    "security:expiring-api-keys",
		Fingerprint: "n=1",
	})
	if err := repo.Dismiss(userID, []uint{settled.ID}); err != nil {
		t.Fatalf("dismiss: %v", err)
	}

	ancient := open.CreatedAt.AddDate(0, 0, 1)
	if _, err := repo.Prune(ancient); err != nil {
		t.Fatalf("prune: %v", err)
	}

	if err := tx.First(&models.Notification{}, open.ID).Error; err != nil {
		t.Fatalf("an open item must survive pruning: %v", err)
	}
	if err := tx.First(&models.Notification{}, settled.ID).Error; err == nil {
		t.Fatal("a settled item older than the cutoff must be pruned")
	}
}
