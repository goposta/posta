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

package worker

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/webhook"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/hibiken/asynq"
	"github.com/jkaninda/logger"
)

// InboundAttachmentView is the attachment shape included in the email.inbound webhook.
// No raw content — only metadata plus a signed download URL (when configured).
type InboundAttachmentView struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	URL         string `json:"url,omitempty"`
}

// InboundWebhookPayload is the body posted to subscribers of email.inbound.
type InboundWebhookPayload struct {
	Event       string                  `json:"event"`
	Timestamp   string                  `json:"timestamp"`
	InboundID   string                  `json:"inbound_id"`
	From        string                  `json:"from"`
	To          []string                `json:"to"`
	Subject     string                  `json:"subject"`
	TextBody    string                  `json:"text_body,omitempty"`
	HTMLBody    string                  `json:"html_body,omitempty"`
	Headers     map[string]string       `json:"headers,omitempty"`
	Attachments []InboundAttachmentView `json:"attachments,omitempty"`
	Size        int64                   `json:"size"`
	MessageID   string                  `json:"message_id,omitempty"`
	Source      string                  `json:"source"`
	ReceivedAt  string                  `json:"received_at"`
}

// InboundProcessHandler processes inbound:process tasks — builds the inbound
// webhook payload and dispatches it via the webhook dispatcher.
type InboundProcessHandler struct {
	repo        *repositories.InboundEmailRepository
	dispatcher  *webhook.Dispatcher
	baseURL     string
	hmacKey     []byte
	onForwarded func()
	onFailed    func()
}

func NewInboundProcessHandler(
	repo *repositories.InboundEmailRepository,
	dispatcher *webhook.Dispatcher,
	baseURL string,
	hmacKey []byte,
) *InboundProcessHandler {
	return &InboundProcessHandler{
		repo:       repo,
		dispatcher: dispatcher,
		baseURL:    strings.TrimRight(baseURL, "/"),
		hmacKey:    hmacKey,
	}
}

// OnForwarded sets a callback invoked after a successful inbound forward.
func (h *InboundProcessHandler) OnForwarded(fn func()) { h.onForwarded = fn }

// OnFailed sets a callback invoked after a permanently failed inbound forward.
func (h *InboundProcessHandler) OnFailed(fn func()) { h.onFailed = fn }

func (h *InboundProcessHandler) ProcessTask(_ context.Context, t *asynq.Task) error {
	var payload InboundProcessPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal inbound payload: %w", err)
	}

	rec, err := h.repo.FindByID(payload.InboundEmailID)
	if err != nil {
		return fmt.Errorf("inbound email not found: %w", err)
	}

	headers := map[string]string{}
	if rec.HeadersJSON != "" {
		_ = json.Unmarshal([]byte(rec.HeadersJSON), &headers)
	}

	var stored []models.InboundAttachmentMeta
	if rec.AttachmentsJSON != "" {
		_ = json.Unmarshal([]byte(rec.AttachmentsJSON), &stored)
	}

	attachments := make([]InboundAttachmentView, 0, len(stored))
	for i, a := range stored {
		view := InboundAttachmentView{
			Filename:    a.Filename,
			ContentType: a.ContentType,
			Size:        a.Size,
		}
		if a.StorageKey != "" && h.baseURL != "" {
			view.URL = h.attachmentURL(rec.UUID, i)
		}
		attachments = append(attachments, view)
	}

	body := InboundWebhookPayload{
		Event:       "email.inbound",
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		InboundID:   rec.UUID,
		From:        rec.Sender,
		To:          []string(rec.Recipients),
		Subject:     rec.Subject,
		TextBody:    rec.TextBody,
		HTMLBody:    rec.HTMLBody,
		Headers:     headers,
		Attachments: attachments,
		Size:        rec.Size,
		MessageID:   rec.MessageID,
		Source:      string(rec.Source),
		ReceivedAt:  rec.ReceivedAt.UTC().Format(time.RFC3339),
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal inbound webhook body: %w", err)
	}

	h.dispatcher.DispatchJSON(rec.UserID, "email.inbound", encoded, rec.Sender)

	now := time.Now().UTC()
	rec.Status = models.InboundStatusForwarded
	rec.ForwardedAt = &now
	rec.ErrorMessage = ""
	if err := h.repo.Update(rec); err != nil {
		logger.Error("failed to mark inbound forwarded", "id", rec.ID, "error", err)
	}

	if h.onForwarded != nil {
		h.onForwarded()
	}
	return nil
}

// attachmentURL builds a signed URL for downloading an inbound attachment.
// Format: {baseURL}/api/v1/inbound/attachments/{uuid}/{idx}?t={token}
func (h *InboundProcessHandler) attachmentURL(uuid string, idx int) string {
	token := SignInboundAttachmentToken(h.hmacKey, uuid, idx)
	return fmt.Sprintf("%s/api/v1/inbound/attachments/%s/%d?t=%s", h.baseURL, uuid, idx, token)
}

// SignInboundAttachmentToken creates an HMAC-signed token authorizing access
// to a specific inbound attachment.
func SignInboundAttachmentToken(key []byte, uuid string, idx int) string {
	payload := uuid + ":" + strconv.Itoa(idx)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// VerifyInboundAttachmentToken checks whether the provided token is valid for
// the given (uuid, idx) pair.
func VerifyInboundAttachmentToken(key []byte, uuid string, idx int, token string) bool {
	expected := SignInboundAttachmentToken(key, uuid, idx)
	return hmac.Equal([]byte(expected), []byte(token))
}

// InboundExhaustedErrorHandler marks inbound emails permanently failed once Asynq
// exhausts retries for the inbound:process task.
type InboundExhaustedErrorHandler struct {
	repo     *repositories.InboundEmailRepository
	onFailed func()
}

func NewInboundExhaustedErrorHandler(repo *repositories.InboundEmailRepository, onFailed func()) *InboundExhaustedErrorHandler {
	return &InboundExhaustedErrorHandler{repo: repo, onFailed: onFailed}
}

// ChainErrorHandlers returns an asynq.ErrorHandler that invokes each handler in
// order. Each inner handler is expected to guard on task type and no-op otherwise.
func ChainErrorHandlers(handlers ...asynq.ErrorHandler) asynq.ErrorHandler {
	return asynq.ErrorHandlerFunc(func(ctx context.Context, t *asynq.Task, err error) {
		for _, h := range handlers {
			h.HandleError(ctx, t, err)
		}
	})
}

func (e *InboundExhaustedErrorHandler) HandleError(_ context.Context, t *asynq.Task, err error) {
	if t.Type() != TypeInboundProcess {
		return
	}
	var payload InboundProcessPayload
	if jerr := json.Unmarshal(t.Payload(), &payload); jerr != nil {
		logger.Error("inbound exhausted: unmarshal", "error", jerr)
		return
	}
	rec, ferr := e.repo.FindByID(payload.InboundEmailID)
	if ferr != nil {
		return
	}
	rec.Status = models.InboundStatusFailed
	rec.ErrorMessage = fmt.Sprintf("permanently failed after retries: %v", err)
	_ = e.repo.Update(rec)
	if e.onFailed != nil {
		e.onFailed()
	}
	logger.Error("worker: inbound forward permanently failed", "id", rec.ID, "error", err)
}
