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

package repositories_test

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/storage/migration"
	"github.com/goposta/posta/internal/storage/repositories"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	migrateOnce sync.Once
	migrateErr  error
)

// migratedDB opens TEST_DATABASE_DSN and runs the full migration once, so these
// tests exercise the real schema, constraints included. Unlike testDB they
// never fall back to a default DSN: the migration rewrites the schema, so it
// must only run against a database that was named for it. Each test works in
// a transaction it rolls back.
func migratedDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Skipf("no test database: %v", err)
	}
	migrateOnce.Do(func() { migrateErr = migration.Run(db) })
	if migrateErr != nil {
		t.Fatalf("migrate: %v", migrateErr)
	}
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })
	return tx
}

type fixture struct {
	user uint
	ws   uint
	form *models.Form
}

func newFixture(t *testing.T, tx *gorm.DB, tag string) fixture {
	t.Helper()
	user := &models.User{Name: "t", Email: tag + "@test.local", PasswordHash: "x"}
	if err := tx.Create(user).Error; err != nil {
		t.Fatalf("user: %v", err)
	}
	ws := &models.Workspace{Name: tag, Slug: tag + fmt.Sprint(time.Now().UnixNano()), OwnerID: user.ID}
	if err := tx.Create(ws).Error; err != nil {
		t.Fatalf("workspace: %v", err)
	}
	form := &models.Form{WorkspaceID: &ws.ID, Name: tag, Slug: tag, PublicKey: tag + fmt.Sprint(time.Now().UnixNano())}
	if err := tx.Create(form).Error; err != nil {
		t.Fatalf("form: %v", err)
	}
	return fixture{user: user.ID, ws: ws.ID, form: form}
}

func (f fixture) message(t *testing.T, tx *gorm.DB, mutate func(*models.Message)) *models.Message {
	t.Helper()
	m := &models.Message{
		WorkspaceID: &f.ws, FormID: f.form.ID, SenderEmail: "ada@example.com",
		Status: models.MessageStatusReceived, State: models.MessageStateNew,
		ThreadToken: fmt.Sprint(time.Now().UnixNano()),
	}
	if mutate != nil {
		mutate(m)
	}
	if err := tx.Create(m).Error; err != nil {
		t.Fatalf("message: %v", err)
	}
	return m
}

func reload(t *testing.T, tx *gorm.DB, id uint) *models.Message {
	t.Helper()
	var m models.Message
	if err := tx.Unscoped().First(&m, id).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	return &m
}

// The bug UpdateFields fixes: the worker loaded a message, spent seconds
// sending notification mail, then saved its stale copy over an operator's
// assignment. Column-scoped writes must leave other columns alone.
func TestUpdateFieldsDoesNotClobberConcurrentChanges(t *testing.T) {
	tx := migratedDB(t)
	f := newFixture(t, tx, "clobber")
	repo := repositories.NewMessageRepository(tx)
	msg := f.message(t, tx, nil)

	assignee := f.user
	if err := repo.UpdateFields(msg.ID, map[string]any{"assigned_to_id": assignee, "state": models.MessageStateClosed}); err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkNotified([]uint{msg.ID}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateFields(msg.ID, map[string]any{"webhook_dispatched_at": time.Now()}); err != nil {
		t.Fatal(err)
	}

	got := reload(t, tx, msg.ID)
	if got.AssignedToID == nil || *got.AssignedToID != assignee || got.State != models.MessageStateClosed {
		t.Fatalf("operator changes lost: assigned=%v state=%s", got.AssignedToID, got.State)
	}
	if got.NotifiedAt == nil || got.WebhookDispatchedAt == nil {
		t.Fatal("worker stamps not written")
	}
}

func TestRecordReplyIsAtomicAndRespectsClosed(t *testing.T) {
	tx := migratedDB(t)
	f := newFixture(t, tx, "reply")
	repo := repositories.NewMessageRepository(tx)
	open := f.message(t, tx, nil)
	closed := f.message(t, tx, func(m *models.Message) { m.State = models.MessageStateClosed })

	first := time.Now().Add(-time.Minute).UTC().Truncate(time.Second)
	if err := repo.RecordReply(open.ID, first); err != nil {
		t.Fatal(err)
	}
	if err := repo.RecordReply(open.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := repo.RecordReply(closed.ID, time.Now()); err != nil {
		t.Fatal(err)
	}

	got := reload(t, tx, open.ID)
	if got.ReplyCount != 2 || got.State != models.MessageStateReplied {
		t.Fatalf("count=%d state=%s, want 2/replied", got.ReplyCount, got.State)
	}
	if got.ReadAt == nil || !got.ReadAt.UTC().Equal(first) {
		t.Fatalf("read_at = %v, want the first reply's time kept", got.ReadAt)
	}
	if c := reload(t, tx, closed.ID); c.State != models.MessageStateClosed || c.ReplyCount != 1 {
		t.Fatalf("closed message: state=%s count=%d", c.State, c.ReplyCount)
	}
}

func TestMarkReadOpensNewOnlyOnce(t *testing.T) {
	tx := migratedDB(t)
	f := newFixture(t, tx, "read")
	repo := repositories.NewMessageRepository(tx)
	fresh := f.message(t, tx, nil)
	closed := f.message(t, tx, func(m *models.Message) { m.State = models.MessageStateClosed })

	at := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
	_ = repo.MarkRead(fresh.ID, at)
	_ = repo.MarkRead(fresh.ID, time.Now())
	_ = repo.MarkRead(closed.ID, time.Now())

	got := reload(t, tx, fresh.ID)
	if got.State != models.MessageStateOpen || got.ReadAt == nil || !got.ReadAt.UTC().Equal(at) {
		t.Fatalf("state=%s read_at=%v", got.State, got.ReadAt)
	}
	if c := reload(t, tx, closed.ID); c.State != models.MessageStateClosed || c.ReadAt == nil {
		t.Fatalf("closed message: state=%s read_at=%v", c.State, c.ReadAt)
	}
}

func TestThreadMessageIDs(t *testing.T) {
	tx := migratedDB(t)
	f := newFixture(t, tx, "thread")
	repo := repositories.NewMessageRepository(tx)
	msg := f.message(t, tx, nil)

	if err := repo.SetRootMessageID(msg.ID, "<root@x>"); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetRootMessageID(msg.ID, "<second@x>"); err != nil {
		t.Fatal(err)
	}
	if got := reload(t, tx, msg.ID).RootMessageID; got != "<root@x>" {
		t.Fatalf("root = %q, want the first writer to win", got)
	}

	base := time.Now().Add(-time.Hour)
	for i, id := range []string{"<root@x>", "", "<b@x>"} {
		r := &models.MessageReply{MessageID: msg.ID, WorkspaceID: &f.ws, Kind: models.MessageReplyKindOperator,
			MessageIDHeader: id, CreatedAt: base.Add(time.Duration(i) * time.Minute)}
		if err := repo.AddReply(r); err != nil {
			t.Fatal(err)
		}
	}
	ids, err := repo.ReplyMessageIDs(msg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != "<root@x>" || ids[1] != "<b@x>" {
		t.Fatalf("ids = %v", ids)
	}
}

func TestAddInboundReplyIsIdempotentPerInboundEmail(t *testing.T) {
	tx := migratedDB(t)
	f := newFixture(t, tx, "inbound")
	repo := repositories.NewMessageRepository(tx)
	msg := f.message(t, tx, nil)
	inboundID := uint(987654)

	reply := func() *models.MessageReply {
		return &models.MessageReply{MessageID: msg.ID, WorkspaceID: &f.ws,
			Kind: models.MessageReplyKindInbound, InboundEmailID: &inboundID}
	}
	created, err := repo.AddInboundReply(reply())
	if err != nil || !created {
		t.Fatalf("first: created=%v err=%v", created, err)
	}
	// The duplicate violates the unique index, which aborts a Postgres
	// transaction; isolate it in a savepoint as a real retry would be isolated.
	tx.SavePoint("dupe")
	created, err = repo.AddInboundReply(reply())
	tx.RollbackTo("dupe")
	if err != nil || created {
		t.Fatalf("duplicate: created=%v err=%v, want false/nil", created, err)
	}
}

func TestDeleteByFormOlderThanIsScopedToTheForm(t *testing.T) {
	tx := migratedDB(t)
	a := newFixture(t, tx, "reta")
	b := newFixture(t, tx, "retb")
	repo := repositories.NewMessageRepository(tx)
	old := time.Now().AddDate(0, 0, -30)

	oldA := a.message(t, tx, func(m *models.Message) { m.CreatedAt = old })
	newA := a.message(t, tx, nil)
	oldB := b.message(t, tx, func(m *models.Message) { m.CreatedAt = old })
	if err := repo.AddReply(&models.MessageReply{MessageID: oldA.ID, WorkspaceID: &a.ws, Kind: models.MessageReplyKindOperator}); err != nil {
		t.Fatal(err)
	}

	n, err := repo.DeleteByFormOlderThan(a.form.ID, time.Now().AddDate(0, 0, -7))
	if err != nil || n != 1 {
		t.Fatalf("deleted %d, err %v; want 1", n, err)
	}
	var left []uint
	tx.Unscoped().Model(&models.Message{}).Where("id IN ?", []uint{oldA.ID, newA.ID, oldB.ID}).Pluck("id", &left)
	if len(left) != 2 {
		t.Fatalf("remaining = %v, want the recent A and the old B", left)
	}
	var replies int64
	tx.Model(&models.MessageReply{}).Where("message_id = ?", oldA.ID).Count(&replies)
	if replies != 0 {
		t.Fatal("replies of a deleted message must cascade")
	}
}

func TestEraseMessagesBySender(t *testing.T) {
	tx := migratedDB(t)
	f := newFixture(t, tx, "erase")
	other := newFixture(t, tx, "eraseother")

	domain := &models.Domain{UserID: f.user, WorkspaceID: &f.ws, Domain: "reply.example.com", VerificationToken: "t"}
	if err := tx.Create(domain).Error; err != nil {
		t.Fatalf("domain: %v", err)
	}
	inbound := &models.InboundEmail{UserID: f.user, WorkspaceID: &f.ws, DomainID: domain.ID, Sender: "ada@example.com",
		Recipients: []string{"msg+tok@reply.example.com"}, RawStorageKey: "inbound/x/raw.eml", AttachmentsJSON: `[{"storage_key":"inbound/x/a.pdf"}]`,
		Status: models.InboundStatusForwarded, Source: models.InboundSourceSMTP}
	if err := tx.Create(inbound).Error; err != nil {
		t.Fatalf("inbound: %v", err)
	}

	target := f.message(t, tx, func(m *models.Message) {
		m.SenderEmail = "Ada@Example.com"
		m.AttachmentsJSON = `[{"storage_key":"messages/m/cv.pdf"}]`
	})
	softDeleted := f.message(t, tx, nil)
	tx.Delete(&models.Message{}, softDeleted.ID)
	keep := f.message(t, tx, func(m *models.Message) { m.SenderEmail = "bob@example.com" })
	otherWS := other.message(t, tx, nil)

	repo := repositories.NewMessageRepository(tx)
	if err := repo.AddReply(&models.MessageReply{MessageID: target.ID, WorkspaceID: &f.ws,
		Kind: models.MessageReplyKindInbound, InboundEmailID: &inbound.ID}); err != nil {
		t.Fatal(err)
	}

	n, keys, err := repositories.EraseMessagesBySender(tx, f.ws, "ADA@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("deleted %d, want 2 (including the soft-deleted one)", n)
	}
	want := map[string]bool{"messages/m/cv.pdf": true, "inbound/x/a.pdf": true, "inbound/x/raw.eml": true}
	if len(keys) != len(want) {
		t.Fatalf("blob keys = %v", keys)
	}
	for _, k := range keys {
		if !want[k] {
			t.Fatalf("unexpected blob key %q", k)
		}
	}

	var count int64
	tx.Unscoped().Model(&models.Message{}).Where("id IN ?", []uint{keep.ID, otherWS.ID}).Count(&count)
	if count != 2 {
		t.Fatal("erasure reached another sender or another workspace")
	}
	tx.Unscoped().Model(&models.InboundEmail{}).Where("id = ?", inbound.ID).Count(&count)
	if count != 0 {
		t.Fatal("the inbound email behind a thread reply must be erased too")
	}
}
