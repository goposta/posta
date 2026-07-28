/*
 * Copyright 2026 Jonas Kaninda
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 */

package email

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/ratelimit"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// stubEnqueuer stands in for the Asynq producer: it records the enqueued email id
// and, crucially, sends nothing itself — exactly like production, where delivery
// happens later in a worker that has only the stored record to work from.
type stubEnqueuer struct{ enqueued []uint }

func (s *stubEnqueuer) EnqueueEmailSend(emailID uint, _ string) error {
	s.enqueued = append(s.enqueued, emailID)
	return nil
}

func (s *stubEnqueuer) EnqueueEmailSendAt(emailID uint, _ string, _ time.Time) error {
	s.enqueued = append(s.enqueued, emailID)
	return nil
}

func pipelineTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		dsn = "host=localhost user=posta password=posta dbname=posta port=5432 sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Skipf("skipping: no test database available: %v", err)
	}
	if err := db.Exec(`CREATE EXTENSION IF NOT EXISTS pgcrypto`).Error; err != nil {
		t.Skipf("skipping: cannot enable pgcrypto: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Email{}); err != nil {
		t.Skipf("skipping: cannot migrate test schema: %v", err)
	}
	return db
}

func pipelineTestLimiter(t *testing.T) *ratelimit.RedisLimiter {
	t.Helper()
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Skipf("skipping: no test redis available: %v", err)
	}
	return ratelimit.NewRedisLimiter(client, 100000, 100000)
}

// Reproduces the production failure behind DashaMail's "Невозможно прочитать файл!":
// an email submitted with an attachment is queued for the worker, and the worker
// only ever sees the stored record. If the payload is not recoverable from that
// record the attachment goes out as a zero-byte file.
//
// This is the SMTP relay's only path — the relay always goes through Send, which
// always enqueues.
func TestQueuedEmailKeepsRecoverableAttachment(t *testing.T) {
	db := pipelineTestDB(t)
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })

	user := &models.User{Name: "relay", Email: "relay-attach@test.local", PasswordHash: "x"}
	if err := tx.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	enq := &stubEnqueuer{}
	svc := &Service{
		emailRepo: repositories.NewEmailRepository(tx),
		limiter:   pipelineTestLimiter(t),
		enqueuer:  enq,
		// blobStore intentionally nil: POSTA_BLOB_PROVIDER is empty by default.
	}

	payload := []byte("delivery check 2026-07-27 12:05:51 +0300\n")
	resp, err := svc.Send(context.Background(), user.ID, 0, nil, user.Email, &SendRequest{
		From:    "noreply@sanatory.ru",
		To:      []string{"rcpt@example.com"},
		Subject: "Проверка вложения",
		Text:    "Файл во вложении",
		Attachments: []models.Attachment{{
			Filename:    "delivery-check-1785143151.txt",
			ContentType: "text/plain",
			Content:     base64.StdEncoding.EncodeToString(payload),
		}},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if resp.Status != models.EmailStatusQueued {
		t.Fatalf("status = %q, want queued", resp.Status)
	}
	if len(enq.enqueued) != 1 {
		t.Fatalf("enqueued %d emails, want 1", len(enq.enqueued))
	}

	// Reload exactly what the worker will read.
	em, err := repositories.NewEmailRepository(tx).FindByID(enq.enqueued[0])
	if err != nil {
		t.Fatalf("reload email: %v", err)
	}

	atts, err := LoadAttachments(context.Background(), em, nil)
	if err != nil {
		t.Fatalf("LoadAttachments: %v", err)
	}
	if len(atts) != 1 {
		t.Fatalf("recovered %d attachments, want 1", len(atts))
	}
	if atts[0].Content == "" {
		t.Fatalf("attachment payload was lost: the worker would send a zero-byte file "+
			"(record attachments_json=%q)", em.AttachmentsJSON)
	}
	got, err := base64.StdEncoding.DecodeString(atts[0].Content)
	if err != nil {
		t.Fatalf("recovered content is not valid base64: %v", err)
	}
	if string(got) != string(payload) {
		t.Errorf("recovered payload = %q, want %q", got, payload)
	}
	if atts[0].Filename != "delivery-check-1785143151.txt" {
		t.Errorf("filename = %q, want delivery-check-1785143151.txt", atts[0].Filename)
	}
	if atts[0].ContentType != "text/plain" {
		t.Errorf("content type = %q, want text/plain", atts[0].ContentType)
	}
}

// With a blob store the bytes go to the store and the record keeps only the key,
// so the payload never sits in the database.
func TestQueuedEmailStoresAttachmentInBlobStore(t *testing.T) {
	db := pipelineTestDB(t)
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })

	user := &models.User{Name: "relay", Email: "relay-blob@test.local", PasswordHash: "x"}
	if err := tx.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	store := &memBlobStore{objects: map[string][]byte{}}
	enq := &stubEnqueuer{}
	svc := &Service{
		emailRepo: repositories.NewEmailRepository(tx),
		limiter:   pipelineTestLimiter(t),
		enqueuer:  enq,
		blobStore: store,
	}

	payload := []byte("delivery check")
	if _, err := svc.Send(context.Background(), user.ID, 0, nil, user.Email, &SendRequest{
		From:    "noreply@sanatory.ru",
		To:      []string{"rcpt@example.com"},
		Subject: "Проверка вложения",
		Text:    "Файл во вложении",
		Attachments: []models.Attachment{{
			Filename:    "delivery-check.txt",
			ContentType: "text/plain",
			Content:     base64.StdEncoding.EncodeToString(payload),
		}},
	}); err != nil {
		t.Fatalf("Send: %v", err)
	}

	em, err := repositories.NewEmailRepository(tx).FindByID(enq.enqueued[0])
	if err != nil {
		t.Fatalf("reload email: %v", err)
	}

	var meta []map[string]any
	if err := json.Unmarshal([]byte(em.AttachmentsJSON), &meta); err != nil {
		t.Fatalf("attachments_json is not valid JSON: %v", err)
	}
	if content, _ := meta[0]["content"].(string); content != "" {
		t.Errorf("payload should live in the blob store, not the record: %s", em.AttachmentsJSON)
	}

	atts, err := LoadAttachments(context.Background(), em, store)
	if err != nil {
		t.Fatalf("LoadAttachments: %v", err)
	}
	got, err := base64.StdEncoding.DecodeString(atts[0].Content)
	if err != nil {
		t.Fatalf("recovered content is not valid base64: %v", err)
	}
	if string(got) != string(payload) {
		t.Errorf("recovered payload = %q, want %q", got, payload)
	}
}

// memBlobStore is an in-memory blob.Store for the blob-backed path.
type memBlobStore struct{ objects map[string][]byte }

func (m *memBlobStore) Put(_ context.Context, key string, r io.Reader, _ string) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	m.objects[key] = data
	return nil
}

func (m *memBlobStore) Get(_ context.Context, key string) (io.ReadCloser, error) {
	data, ok := m.objects[key]
	if !ok {
		return nil, errors.New("not found")
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (m *memBlobStore) Delete(_ context.Context, key string) error {
	delete(m.objects, key)
	return nil
}

func (m *memBlobStore) Exists(_ context.Context, key string) (bool, error) {
	_, ok := m.objects[key]
	return ok, nil
}
