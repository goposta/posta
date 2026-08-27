// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package repositories

import (
	"testing"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/lib/pq"
)

func TestScrubOlderThan_ClearsContentKeepsMetadata(t *testing.T) {
	db := testDB(t)
	if err := db.AutoMigrate(&models.Email{}); err != nil {
		t.Skipf("skipping: cannot migrate email schema: %v", err)
	}

	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })

	userID := createUser(t, tx, "scrub@test.local")
	repo := NewEmailRepository(tx)

	old := &models.Email{
		UserID:          userID,
		Sender:          "from@test.local",
		Recipients:      pq.StringArray{"to@test.local"},
		Subject:         "keep me",
		HTMLBody:        "<p>secret</p>",
		TextBody:        "secret",
		AttachmentsJSON: `[{"filename":"a.pdf"}]`,
		Status:          models.EmailStatusSent,
		CreatedAt:       time.Now().AddDate(0, 0, -60),
	}
	recent := &models.Email{
		UserID:     userID,
		Sender:     "from@test.local",
		Recipients: pq.StringArray{"to@test.local"},
		Subject:    "recent",
		HTMLBody:   "<p>fresh</p>",
		TextBody:   "fresh",
		Status:     models.EmailStatusSent,
		CreatedAt:  time.Now(),
	}
	if err := repo.Create(old); err != nil {
		t.Fatalf("create old: %v", err)
	}
	if err := repo.Create(recent); err != nil {
		t.Fatalf("create recent: %v", err)
	}

	cutoff := time.Now().AddDate(0, 0, -30)
	if n, err := repo.ScrubBodiesOlderThan(cutoff); err != nil || n != 1 {
		t.Fatalf("ScrubBodiesOlderThan: n=%d err=%v (want 1, nil)", n, err)
	}
	if n, err := repo.ScrubAttachmentsOlderThan(cutoff); err != nil || n != 1 {
		t.Fatalf("ScrubAttachmentsOlderThan: n=%d err=%v (want 1, nil)", n, err)
	}

	gotOld, err := repo.FindByID(old.ID)
	if err != nil {
		t.Fatalf("reload old: %v", err)
	}
	if gotOld.HTMLBody != "" || gotOld.TextBody != "" || gotOld.AttachmentsJSON != "" {
		t.Fatalf("old content not scrubbed: html=%q text=%q att=%q", gotOld.HTMLBody, gotOld.TextBody, gotOld.AttachmentsJSON)
	}
	if gotOld.Subject != "keep me" || gotOld.Sender != "from@test.local" {
		t.Fatalf("old metadata altered: subject=%q sender=%q", gotOld.Subject, gotOld.Sender)
	}

	gotRecent, err := repo.FindByID(recent.ID)
	if err != nil {
		t.Fatalf("reload recent: %v", err)
	}
	if gotRecent.HTMLBody == "" || gotRecent.TextBody == "" {
		t.Fatalf("recent content should be untouched, got html=%q text=%q", gotRecent.HTMLBody, gotRecent.TextBody)
	}
}
