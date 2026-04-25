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

package handlers

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/eventbus"
	"github.com/goposta/posta/internal/services/inbound"
	"github.com/goposta/posta/internal/storage/blob"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/goposta/posta/internal/worker"
	"github.com/jkaninda/logger"
	"github.com/jkaninda/okapi"
)

// InboundHandler serves inbound-email HTTP endpoints: the generic webhook ingest
// endpoint, list/get for authenticated users, and attachment downloads.
type InboundHandler struct {
	svc       *inbound.Service
	repo      *repositories.InboundEmailRepository
	blobStore blob.Store
	producer  inbound.Enqueuer
	bus       *eventbus.EventBus
	secret    string
	hmacKey   []byte
}

func NewInboundHandler(
	svc *inbound.Service,
	repo *repositories.InboundEmailRepository,
	blobStore blob.Store,
	secret string,
	hmacKey []byte,
) *InboundHandler {
	return &InboundHandler{
		svc:       svc,
		repo:      repo,
		blobStore: blobStore,
		secret:    secret,
		hmacKey:   hmacKey,
	}
}

// SetEnqueuer configures the worker enqueuer for the retry endpoint.
func (h *InboundHandler) SetEnqueuer(eq inbound.Enqueuer) { h.producer = eq }

// SetEventBus configures the event bus used for SSE streaming.
func (h *InboundHandler) SetEventBus(b *eventbus.EventBus) { h.bus = b }

// Stream pushes email.inbound.received events to the authenticated user via SSE.
// Events from other users' inboxes are filtered out.
func (h *InboundHandler) Stream(c *okapi.Context) error {
	if h.bus == nil {
		return c.AbortNotFound("inbound stream not configured")
	}
	ctx := c.Request().Context()
	scope := getScope(c)

	ch, unsub := h.bus.Subscribe()
	defer unsub()

	msgCh := make(chan okapi.Message, 4)
	msgCh <- okapi.Message{
		Event: "system.info",
		Data: okapi.M{
			"user_id":   scope.UserID,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		},
	}

	go func() {
		defer close(msgCh)
		for {
			select {
			case <-ctx.Done():
				return
			case evt, ok := <-ch:
				if !ok {
					return
				}
				if evt.Type != "email.inbound.received" && evt.Type != "email.inbound.forwarded" && evt.Type != "email.inbound.failed" {
					continue
				}
				if evt.ActorID == nil || *evt.ActorID != scope.UserID {
					continue
				}
				select {
				case msgCh <- okapi.Message{Event: evt.Type, Data: evt}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return c.SSEStreamWithOptions(ctx, msgCh, &okapi.StreamOptions{
		Serializer:   &okapi.JSONSerializer{},
		PingInterval: 30 * time.Second,
	})
}

// contentDisposition builds a safe Content-Disposition header. Non-ASCII filenames
// are RFC 5987 encoded; quotes and CR/LF are stripped from the fallback name.
func contentDisposition(filename string) string {
	ascii := strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || r == '"' || r == '\\' || r > 0x7e {
			return -1
		}
		return r
	}, filename)
	if ascii == "" {
		ascii = "attachment"
	}
	return `attachment; filename="` + ascii + `"; filename*=UTF-8''` + urlPathEscape(filename)
}

// urlPathEscape percent-encodes a filename per RFC 5987 / 3986 unreserved.
func urlPathEscape(s string) string {
	const unreserved = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_.~"
	var b strings.Builder
	for _, c := range []byte(s) {
		if strings.IndexByte(unreserved, c) >= 0 {
			b.WriteByte(c)
		} else {
			b.WriteByte('%')
			const hexd = "0123456789ABCDEF"
			b.WriteByte(hexd[c>>4])
			b.WriteByte(hexd[c&0x0f])
		}
	}
	return b.String()
}

// constantTimeSecretEqual compares two secrets in constant time, including length.
func constantTimeSecretEqual(a, b string) bool {
	if len(a) != len(b) {
		// still run the compare to equalize timing
		_ = subtle.ConstantTimeCompare([]byte(a), []byte(a))
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// DedupFallbackHash derives a stable hash for messages without a Message-ID.
// Used by the webhook path to reject near-duplicate retries.
func DedupFallbackHash(sender string, recipients []string, subject string, size int64) string {
	h := sha256.New()
	h.Write([]byte(strings.ToLower(sender)))
	h.Write([]byte{0})
	for _, r := range recipients {
		h.Write([]byte(strings.ToLower(r)))
		h.Write([]byte{0})
	}
	h.Write([]byte(subject))
	h.Write([]byte{0})
	h.Write([]byte(mime.QEncoding.Encode("utf-8", "") + hex.EncodeToString([]byte{byte(size), byte(size >> 8), byte(size >> 16), byte(size >> 24)})))
	return "fh-" + hex.EncodeToString(h.Sum(nil))[:24]
}

// InboundWebhookAttachment is the normalized attachment shape accepted on the
// generic webhook — content is base64-encoded. Providers (Mailgun/SendGrid/SES)
// can be adapted upstream to this shape.
type InboundWebhookAttachment struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Content     string `json:"content" doc:"Base64-encoded content"`
}

// InboundWebhookRequest is the normalized inbound-email payload accepted on the
// public webhook endpoint.
type InboundWebhookRequest struct {
	PostaInboundSecret string `header:"X-Posta-Inbound-Secret" required:"true"`
	Body               struct {
		From        string                     `json:"from" required:"true"`
		To          []string                   `json:"to" required:"true"`
		Subject     string                     `json:"subject"`
		Text        string                     `json:"text"`
		HTML        string                     `json:"html"`
		Headers     map[string]string          `json:"headers"`
		MessageID   string                     `json:"message_id"`
		SpamScore   *float64                   `json:"spam_score,omitempty"`
		Attachments []InboundWebhookAttachment `json:"attachments"`
		Raw         string                     `json:"raw" doc:"Optional base64-encoded raw RFC 5322 message"`
	} `json:"body"`
}

type InboundWebhookResponse struct {
	Accepted  bool   `json:"accepted"`
	InboundID string `json:"inbound_id,omitempty"`
	Status    string `json:"status"`
}

// Receive accepts a normalized inbound email from an external MX provider.
// Auth is a shared secret provided via X-Posta-Inbound-Secret header.
func (h *InboundHandler) Receive(c *okapi.Context, req *InboundWebhookRequest) error {
	if h.secret == "" {
		return c.AbortForbidden("inbound webhook is not configured")
	}
	if !constantTimeSecretEqual(req.PostaInboundSecret, h.secret) {
		return c.AbortUnauthorized("invalid inbound secret")
	}

	parsed := &inbound.ParsedEmail{
		MessageID: strings.Trim(req.Body.MessageID, "<>"),
		From:      req.Body.From,
		To:        req.Body.To,
		Subject:   req.Body.Subject,
		TextBody:  req.Body.Text,
		HTMLBody:  req.Body.HTML,
		Headers:   req.Body.Headers,
		Date:      time.Now().UTC(),
	}
	if req.Body.Raw != "" {
		if raw, err := base64.StdEncoding.DecodeString(req.Body.Raw); err == nil {
			parsed.Raw = raw
		}
	}
	for _, a := range req.Body.Attachments {
		decoded, err := base64.StdEncoding.DecodeString(a.Content)
		if err != nil {
			return c.AbortBadRequest("invalid base64 in attachment " + a.Filename)
		}
		parsed.Attachments = append(parsed.Attachments, inbound.ParsedAttachment{
			Filename:    a.Filename,
			ContentType: a.ContentType,
			Content:     decoded,
			Size:        int64(len(decoded)),
		})
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 30*time.Second)
	defer cancel()

	rec, err := h.svc.Ingest(ctx, parsed, models.InboundSourceWebhook)
	switch {
	case err == nil:
		return c.JSON(http.StatusAccepted, InboundWebhookResponse{Accepted: true, InboundID: rec.UUID, Status: "received"})
	case errors.Is(err, inbound.ErrDuplicate):
		return c.JSON(http.StatusOK, InboundWebhookResponse{Accepted: true, InboundID: rec.UUID, Status: "duplicate"})
	case errors.Is(err, inbound.ErrUnverifiedDomain):
		return c.AbortForbidden("recipient domain is not verified")
	case errors.Is(err, inbound.ErrSizeExceeded), errors.Is(err, inbound.ErrAttachmentTooLarge):
		return c.JSON(http.StatusRequestEntityTooLarge, InboundWebhookResponse{Accepted: false, Status: "too_large"})
	case errors.Is(err, inbound.ErrSenderSuppressed):
		uuid := ""
		if rec != nil {
			uuid = rec.UUID
		}
		return c.JSON(http.StatusAccepted, InboundWebhookResponse{Accepted: true, InboundID: uuid, Status: "suppressed"})
	default:
		return c.AbortInternalServerError("failed to ingest inbound email", err)
	}
}

// InboundListRequest extends ListRequest with filter query params.
type InboundListRequest struct {
	Page   int    `query:"page" default:"0"`
	Size   int    `query:"size" default:"20"`
	Status string `query:"status" doc:"Filter by status: received|forwarded|failed|rejected"`
	Source string `query:"source" doc:"Filter by source: smtp|webhook"`
	Sender string `query:"sender" doc:"Filter by sender address (substring, case-insensitive)"`
	Q      string `query:"q" doc:"Full-text search on subject"`
}

// List returns inbound emails for the current scope, with optional filters.
func (h *InboundHandler) List(c *okapi.Context, req *InboundListRequest) error {
	page, size, offset := normalizePageParams(req.Page, req.Size)

	items, total, err := h.repo.FindByScopeFiltered(getScope(c), repositories.InboundFilter{
		Status: req.Status,
		Source: req.Source,
		Sender: req.Sender,
		Query:  req.Q,
	}, size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list inbound emails", err)
	}
	return paginated(c, items, total, page, size)
}

// Delete removes an inbound email by UUID (scope-checked) and cleans up blob-stored content.
func (h *InboundHandler) Delete(c *okapi.Context, req *GetEmailRequest) error {
	rec, err := h.repo.FindByUUID(req.ID)
	if err != nil {
		return c.AbortNotFound("inbound email not found")
	}
	if !ownsResource(c, rec.UserID, rec.WorkspaceID) {
		return c.AbortNotFound("inbound email not found")
	}
	// Best-effort blob cleanup.
	if h.blobStore != nil {
		if rec.AttachmentsJSON != "" {
			var atts []models.InboundAttachmentMeta
			if err := json.Unmarshal([]byte(rec.AttachmentsJSON), &atts); err == nil {
				for _, a := range atts {
					if a.StorageKey != "" {
						_ = h.blobStore.Delete(c.Request().Context(), a.StorageKey)
					}
				}
			}
		}
		if rec.RawStorageKey != "" {
			_ = h.blobStore.Delete(c.Request().Context(), rec.RawStorageKey)
		}
	}
	if err := h.repo.Delete(rec.ID); err != nil {
		return c.AbortInternalServerError("failed to delete inbound email", err)
	}
	return noContent(c)
}

// Retry re-enqueues a failed inbound email for another webhook-dispatch attempt.
func (h *InboundHandler) Retry(c *okapi.Context, req *GetEmailRequest) error {
	if h.producer == nil {
		return c.AbortForbidden("inbound retry requires an async worker")
	}
	rec, err := h.repo.FindByUUID(req.ID)
	if err != nil {
		return c.AbortNotFound("inbound email not found")
	}
	if !ownsResource(c, rec.UserID, rec.WorkspaceID) {
		return c.AbortNotFound("inbound email not found")
	}
	if rec.Status != models.InboundStatusFailed && rec.Status != models.InboundStatusReceived {
		return c.AbortBadRequest("only failed or stuck received messages can be retried")
	}
	rec.Status = models.InboundStatusReceived
	rec.ErrorMessage = ""
	if err := h.repo.Update(rec); err != nil {
		return c.AbortInternalServerError("failed to update inbound email", err)
	}
	if err := h.producer.EnqueueInboundProcess(rec.ID); err != nil {
		return c.AbortInternalServerError("failed to enqueue inbound task", err)
	}
	return ok(c, map[string]string{"id": rec.UUID, "status": string(rec.Status)})
}

// DownloadAttachmentAuthed streams an inbound attachment to an authenticated user
// who owns the record — no signed token required.
func (h *InboundHandler) DownloadAttachmentAuthed(c *okapi.Context, req *InboundAttachmentOwnedRequest) error {
	rec, err := h.repo.FindByUUID(req.UUID)
	if err != nil {
		return c.AbortNotFound("inbound email not found")
	}
	if !ownsResource(c, rec.UserID, rec.WorkspaceID) {
		return c.AbortNotFound("inbound email not found")
	}
	return h.streamAttachment(c, rec, req.Index)
}

// InboundAttachmentOwnedRequest is the path-params-only variant used by authed users.
type InboundAttachmentOwnedRequest struct {
	UUID  string `param:"uuid"`
	Index int    `param:"idx"`
}

// GetRaw streams the raw RFC 5322 message bytes (if stored).
func (h *InboundHandler) GetRaw(c *okapi.Context, req *GetEmailRequest) error {
	rec, err := h.repo.FindByUUID(req.ID)
	if err != nil {
		return c.AbortNotFound("inbound email not found")
	}
	if !ownsResource(c, rec.UserID, rec.WorkspaceID) {
		return c.AbortNotFound("inbound email not found")
	}
	if rec.RawStorageKey == "" || h.blobStore == nil {
		return c.AbortNotFound("raw message not available")
	}
	rc, err := h.blobStore.Get(c.Request().Context(), rec.RawStorageKey)
	if err != nil {
		return c.AbortInternalServerError("failed to fetch raw message", err)
	}
	defer func() { _ = rc.Close() }()
	c.ResponseWriter().Header().Set("Content-Type", "message/rfc822")
	c.ResponseWriter().Header().Set("Content-Disposition", contentDisposition(rec.UUID+".eml"))
	c.ResponseWriter().WriteHeader(http.StatusOK)
	if _, err := io.Copy(c.ResponseWriter(), rc); err != nil {
		logger.Warn("inbound raw stream failed", "uuid", rec.UUID, "error", err)
	}
	return nil
}

// streamAttachment is the shared body for both signed and authed attachment endpoints.
func (h *InboundHandler) streamAttachment(c *okapi.Context, rec *models.InboundEmail, idx int) error {
	var atts []models.InboundAttachmentMeta
	if rec.AttachmentsJSON != "" {
		if err := json.Unmarshal([]byte(rec.AttachmentsJSON), &atts); err != nil {
			return c.AbortInternalServerError("failed to decode attachment metadata", err)
		}
	}
	if idx < 0 || idx >= len(atts) {
		return c.AbortNotFound("attachment not found")
	}
	meta := atts[idx]
	c.ResponseWriter().Header().Set("Content-Type", meta.ContentType)
	if meta.Filename != "" {
		c.ResponseWriter().Header().Set("Content-Disposition", contentDisposition(meta.Filename))
	}
	if meta.StorageKey != "" && h.blobStore != nil {
		rc, err := h.blobStore.Get(c.Request().Context(), meta.StorageKey)
		if err != nil {
			return c.AbortInternalServerError("failed to fetch attachment", err)
		}
		defer func() { _ = rc.Close() }()
		c.ResponseWriter().WriteHeader(http.StatusOK)
		if _, err := io.Copy(c.ResponseWriter(), rc); err != nil {
			// Response already committed — log and return nil since headers are flushed.
			logger.Warn("inbound attachment stream failed", "uuid", rec.UUID, "error", err)
		}
		return nil
	}
	if meta.Content != "" {
		raw, err := base64.StdEncoding.DecodeString(meta.Content)
		if err != nil {
			return c.AbortInternalServerError("failed to decode inline attachment", err)
		}
		c.ResponseWriter().WriteHeader(http.StatusOK)
		_, _ = c.ResponseWriter().Write(raw)
		return nil
	}
	return c.AbortNotFound("attachment content unavailable")
}

// Get returns a single inbound email by UUID (scope-checked).
func (h *InboundHandler) Get(c *okapi.Context, req *GetEmailRequest) error {
	rec, err := h.repo.FindByUUID(req.ID)
	if err != nil {
		return c.AbortNotFound("inbound email not found")
	}
	if !ownsResource(c, rec.UserID, rec.WorkspaceID) {
		return c.AbortNotFound("inbound email not found")
	}
	return ok(c, rec)
}

// InboundAttachmentRequest identifies a specific attachment on an inbound email.
type InboundAttachmentRequest struct {
	UUID  string `param:"uuid"`
	Index int    `param:"idx"`
	Token string `query:"t"`
}

// ServeAttachment streams an inbound attachment from blob storage after validating
// the HMAC-signed token. Used by webhook consumers to fetch bytes asynchronously.
func (h *InboundHandler) ServeAttachment(c *okapi.Context, req *InboundAttachmentRequest) error {
	if req.Token == "" || !worker.VerifyInboundAttachmentToken(h.hmacKey, req.UUID, req.Index, req.Token) {
		return c.AbortUnauthorized("invalid or missing token")
	}
	rec, err := h.repo.FindByUUID(req.UUID)
	if err != nil {
		return c.AbortNotFound("inbound email not found")
	}
	var atts []models.InboundAttachmentMeta
	if rec.AttachmentsJSON != "" {
		if err := json.Unmarshal([]byte(rec.AttachmentsJSON), &atts); err != nil {
			return c.AbortInternalServerError("failed to decode attachment metadata", err)
		}
	}
	if req.Index < 0 || req.Index >= len(atts) {
		return c.AbortNotFound("attachment not found")
	}
	meta := atts[req.Index]
	c.ResponseWriter().Header().Set("Content-Type", meta.ContentType)
	if meta.Filename != "" {
		c.ResponseWriter().Header().Set("Content-Disposition", contentDisposition(meta.Filename))
	}

	if meta.StorageKey != "" && h.blobStore != nil {
		rc, err := h.blobStore.Get(c.Request().Context(), meta.StorageKey)
		if err != nil {
			return c.AbortInternalServerError("failed to fetch attachment", err)
		}
		defer func() { _ = rc.Close() }()
		c.ResponseWriter().WriteHeader(http.StatusOK)
		if _, err := io.Copy(c.ResponseWriter(), rc); err != nil {
			logger.Warn("inbound attachment stream failed", "uuid", req.UUID, "error", err)
		}
		return nil
	}
	if meta.Content != "" {
		raw, err := base64.StdEncoding.DecodeString(meta.Content)
		if err != nil {
			return c.AbortInternalServerError("failed to decode inline attachment", err)
		}
		c.ResponseWriter().WriteHeader(http.StatusOK)
		_, _ = c.ResponseWriter().Write(raw)
		return nil
	}
	return c.AbortNotFound("attachment content unavailable")
}
