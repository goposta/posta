/*
 * Copyright 2026 Jonas Kaninda
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package inbound

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/eventbus"
	"github.com/goposta/posta/internal/storage/blob"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/logger"
	"gorm.io/gorm"
)

var (
	// ErrUnverifiedDomain is returned when none of the recipient domains match an
	// ownership-verified domain. Maps to SMTP 550.
	ErrUnverifiedDomain = errors.New("recipient domain is not verified")
	// ErrSizeExceeded is returned when the raw message exceeds the configured limit.
	// Maps to SMTP 552.
	ErrSizeExceeded = errors.New("message size exceeds limit")
	// ErrAttachmentTooLarge is returned when a single attachment is larger than
	// the configured per-attachment limit.
	ErrAttachmentTooLarge = errors.New("attachment exceeds per-attachment size limit")
	// ErrDuplicate is returned when a message with the same Message-ID already
	// exists for the resolved user. Callers should treat this as success (idempotent).
	ErrDuplicate = errors.New("duplicate message-id")
	// ErrSenderSuppressed is returned when the sender is on the recipient's
	// suppression list.
	ErrSenderSuppressed = errors.New("sender is suppressed")
)

// Enqueuer enqueues inbound-processing tasks. Satisfied by worker.Producer.
type Enqueuer interface {
	EnqueueInboundProcess(inboundEmailID uint) error
}

type Service struct {
	repo            *repositories.InboundEmailRepository
	domainRepo      *repositories.DomainRepository
	suppressionRepo *repositories.SuppressionRepository
	blobStore       blob.Store
	producer        Enqueuer
	bus             *eventbus.EventBus
	maxMsgSize      int64
	maxAttachSize   int64
	onReceived      func(source models.InboundSource)
	onRejected      func(reason string)
	onBytes         func(int64)
	onIngestMs      func(float64)
}

// Config holds runtime configuration for the inbound service.
type Config struct {
	MaxMessageSize    int64
	MaxAttachmentSize int64
}

func NewService(
	repo *repositories.InboundEmailRepository,
	domainRepo *repositories.DomainRepository,
	suppressionRepo *repositories.SuppressionRepository,
	cfg Config,
) *Service {
	return &Service{
		repo:            repo,
		domainRepo:      domainRepo,
		suppressionRepo: suppressionRepo,
		maxMsgSize:      cfg.MaxMessageSize,
		maxAttachSize:   cfg.MaxAttachmentSize,
	}
}

// SetBlobStore configures the blob store for attachment persistence.
// When nil, attachment content is stored inline (base64) on the InboundEmail record.
func (s *Service) SetBlobStore(bs blob.Store) { s.blobStore = bs }

// SetEnqueuer configures the worker-task enqueuer.
func (s *Service) SetEnqueuer(eq Enqueuer) { s.producer = eq }

// SetEventBus configures the event bus used to publish inbound.received events.
func (s *Service) SetEventBus(b *eventbus.EventBus) { s.bus = b }

// OnReceived sets a callback invoked after each successfully ingested inbound message.
func (s *Service) OnReceived(fn func(source models.InboundSource)) { s.onReceived = fn }

// OnRejected sets a callback invoked after each rejected inbound message.
func (s *Service) OnRejected(fn func(reason string)) { s.onRejected = fn }

// OnBytes sets a callback invoked with the total inbound byte count.
func (s *Service) OnBytes(fn func(int64)) { s.onBytes = fn }

// OnIngestDuration sets a callback invoked with ingestion duration in seconds.
func (s *Service) OnIngestDuration(fn func(float64)) { s.onIngestMs = fn }

// computeDedupHash derives a stable hash for messages without a Message-ID, used
// on the webhook path to reject near-duplicate retries.
func computeDedupHash(sender string, recipients []string, subject string, size int64) string {
	return "fh-" + fmt.Sprintf("%s|%s|%s|%d", strings.ToLower(sender), strings.ToLower(strings.Join(recipients, ",")), subject, size)
}

// Ingest is the single entry point for both SMTP and webhook paths.
// It resolves a verified tenant domain from the recipient list, persists the
// record (and attachments to blob storage when configured), and enqueues an
// async task to dispatch the email.inbound webhook.
func (s *Service) Ingest(ctx context.Context, p *ParsedEmail, source models.InboundSource) (*models.InboundEmail, error) {
	start := time.Now()
	defer func() {
		if s.onIngestMs != nil {
			s.onIngestMs(time.Since(start).Seconds())
		}
	}()

	if s.maxMsgSize > 0 && int64(len(p.Raw)) > s.maxMsgSize {
		s.rejected("size_exceeded")
		return nil, ErrSizeExceeded
	}

	domain, recipient, err := s.resolveDomain(p.To)
	if err != nil {
		s.rejected("unverified_domain")
		return nil, err
	}

	// Suppression check — skip delivery (record as rejected) if sender is suppressed.
	if s.suppressionRepo != nil && p.From != "" {
		scope := repositories.ResourceScope{UserID: domain.UserID, WorkspaceID: domain.WorkspaceID}
		if yes, _ := s.suppressionRepo.IsSuppressed(scope, strings.ToLower(p.From)); yes {
			s.rejected("sender_suppressed")
			return s.persistRejected(p, domain, source, "sender is on suppression list"), ErrSenderSuppressed
		}
	}

	// Idempotency: first by Message-ID when present, else by content-hash fallback.
	if p.MessageID != "" {
		if existing, err := s.repo.FindByMessageID(domain.UserID, p.MessageID); err == nil && existing != nil {
			return existing, ErrDuplicate
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Warn("failed to check message-id dedup", "error", err)
		}
	}

	recipients := p.To
	if len(recipients) == 0 && recipient != "" {
		recipients = []string{recipient}
	}

	var dedupHash string
	if p.MessageID == "" {
		dedupHash = computeDedupHash(p.From, recipients, p.Subject, int64(len(p.Raw)))
		if existing, err := s.repo.FindByDedupHash(domain.UserID, dedupHash); err == nil && existing != nil {
			return existing, ErrDuplicate
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Warn("failed to check dedup hash", "error", err)
		}
	}

	headersJSON, _ := json.Marshal(p.Headers)

	rec := &models.InboundEmail{
		UserID:      domain.UserID,
		WorkspaceID: domain.WorkspaceID,
		DomainID:    domain.ID,
		MessageID:   p.MessageID,
		DedupHash:   dedupHash,
		Sender:      p.From,
		Recipients:  recipients,
		Subject:     p.Subject,
		TextBody:    p.TextBody,
		HTMLBody:    p.HTMLBody,
		HeadersJSON: string(headersJSON),
		Size:        int64(len(p.Raw)),
		Status:      models.InboundStatusReceived,
		Source:      source,
		ReceivedAt:  time.Now().UTC(),
	}

	if err := s.repo.Create(rec); err != nil {
		return nil, fmt.Errorf("persist inbound email: %w", err)
	}

	// Persist raw .eml bytes (best effort) so operators can export later.
	if s.blobStore != nil && len(p.Raw) > 0 {
		rawKey := fmt.Sprintf("inbound/%s/raw.eml", rec.UUID)
		if err := s.blobStore.Put(ctx, rawKey, bytes.NewReader(p.Raw), "message/rfc822"); err == nil {
			rec.RawStorageKey = rawKey
			_ = s.repo.Update(rec)
		} else {
			logger.Warn("failed to store raw inbound eml", "uuid", rec.UUID, "error", err)
		}
	}

	attachments, uploadedKeys, err := s.persistAttachments(ctx, rec.UUID, p.Attachments)
	if err != nil {
		// Best-effort cleanup of partial uploads so blob storage doesn't leak.
		if s.blobStore != nil {
			for _, k := range uploadedKeys {
				_ = s.blobStore.Delete(ctx, k)
			}
			if rec.RawStorageKey != "" {
				_ = s.blobStore.Delete(ctx, rec.RawStorageKey)
				rec.RawStorageKey = ""
			}
		}
		rec.Status = models.InboundStatusFailed
		rec.ErrorMessage = err.Error()
		_ = s.repo.Update(rec)
		s.rejected("attachment_error")
		return rec, err
	}
	if len(attachments) > 0 {
		if data, mErr := json.Marshal(attachments); mErr == nil {
			rec.AttachmentsJSON = string(data)
			_ = s.repo.Update(rec)
		}
	}

	if s.producer != nil {
		if err := s.producer.EnqueueInboundProcess(rec.ID); err != nil {
			logger.Error("failed to enqueue inbound:process", "inbound_id", rec.ID, "error", err)
		}
	}

	if s.bus != nil {
		actor := rec.UserID
		s.bus.PublishSimple(
			models.EventCategoryEmail,
			"email.inbound.received",
			&actor,
			"",
			"",
			fmt.Sprintf("Inbound email received from %s", rec.Sender),
			map[string]any{
				"inbound_id":   rec.UUID,
				"sender":       rec.Sender,
				"recipients":   []string(rec.Recipients),
				"subject":      rec.Subject,
				"source":       string(rec.Source),
				"size":         rec.Size,
				"workspace_id": rec.WorkspaceID,
			},
		)
	}

	if s.onReceived != nil {
		s.onReceived(source)
	}
	if s.onBytes != nil {
		s.onBytes(rec.Size)
	}
	return rec, nil
}

// resolveDomain walks the recipient list and returns the first ownership-verified
// tenant domain that matches. Returns ErrUnverifiedDomain if none match.
func (s *Service) resolveDomain(recipients []string) (*models.Domain, string, error) {
	for _, r := range recipients {
		addr := strings.ToLower(strings.TrimSpace(r))
		at := strings.LastIndex(addr, "@")
		if at < 0 || at == len(addr)-1 {
			continue
		}
		d, err := s.domainRepo.FindVerifiedByName(addr[at+1:])
		if err != nil {
			continue
		}
		return d, addr, nil
	}
	return nil, "", ErrUnverifiedDomain
}

// persistAttachments stores attachment content in blob storage (or inline when
// no store is configured). Returns attachment metadata plus the list of blob keys
// that were successfully uploaded — the caller uses the keys to clean up on
// later failure.
func (s *Service) persistAttachments(ctx context.Context, uuid string, atts []ParsedAttachment) ([]models.InboundAttachmentMeta, []string, error) {
	if len(atts) == 0 {
		return nil, nil, nil
	}
	out := make([]models.InboundAttachmentMeta, 0, len(atts))
	keys := make([]string, 0, len(atts))
	for i, a := range atts {
		if s.maxAttachSize > 0 && a.Size > s.maxAttachSize {
			return nil, keys, fmt.Errorf("%w: %q (%d bytes)", ErrAttachmentTooLarge, a.Filename, a.Size)
		}
		meta := models.InboundAttachmentMeta{
			Filename:    a.Filename,
			ContentType: a.ContentType,
			Size:        a.Size,
		}
		if s.blobStore != nil {
			key := fmt.Sprintf("inbound/%s/%d_%s", uuid, i, sanitizeFilename(a.Filename))
			if err := s.blobStore.Put(ctx, key, bytes.NewReader(a.Content), a.ContentType); err != nil {
				return nil, keys, fmt.Errorf("store attachment %q: %w", a.Filename, err)
			}
			meta.StorageKey = key
			keys = append(keys, key)
		} else {
			meta.Content = base64.StdEncoding.EncodeToString(a.Content)
		}
		out = append(out, meta)
	}
	return out, keys, nil
}

func (s *Service) persistRejected(p *ParsedEmail, domain *models.Domain, source models.InboundSource, reason string) *models.InboundEmail {
	rec := &models.InboundEmail{
		UserID:       domain.UserID,
		WorkspaceID:  domain.WorkspaceID,
		DomainID:     domain.ID,
		MessageID:    p.MessageID,
		Sender:       p.From,
		Recipients:   p.To,
		Subject:      p.Subject,
		Size:         int64(len(p.Raw)),
		Status:       models.InboundStatusRejected,
		Source:       source,
		ReceivedAt:   time.Now().UTC(),
		ErrorMessage: reason,
	}
	if err := s.repo.Create(rec); err != nil {
		logger.Error("failed to persist rejected inbound email", "error", err)
		return nil
	}
	return rec
}

func (s *Service) rejected(reason string) {
	if s.onRejected != nil {
		s.onRejected(reason)
	}
}

const defaultAttachmentName = "attachment"

func sanitizeFilename(name string) string {
	if name == "" {
		return defaultAttachmentName
	}
	out := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			return r
		}
		return '_'
	}, name)
	if out == "" {
		return defaultAttachmentName
	}
	return out
}
