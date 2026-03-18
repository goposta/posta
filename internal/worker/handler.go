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
	"encoding/json"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jkaninda/logger"
	"github.com/jkaninda/posta/internal/models"
	"github.com/jkaninda/posta/internal/services/email"
	"github.com/jkaninda/posta/internal/services/webhook"
	"github.com/jkaninda/posta/internal/storage/repositories"
)

// EmailSendHandler processes email:send tasks from the Asynq queue.
type EmailSendHandler struct {
	emailRepo   *repositories.EmailRepository
	smtpRepo    *repositories.SMTPRepository
	serverRepo  *repositories.ServerRepository
	domainRepo  *repositories.DomainRepository
	contactRepo *repositories.ContactRepository
	sender      *email.SMTPSender
	dispatcher  *webhook.Dispatcher
	onSent      func()
	onFailed    func()
}

func NewEmailSendHandler(
	emailRepo *repositories.EmailRepository,
	smtpRepo *repositories.SMTPRepository,
	serverRepo *repositories.ServerRepository,
	domainRepo *repositories.DomainRepository,
	contactRepo *repositories.ContactRepository,
	dispatcher *webhook.Dispatcher,
) *EmailSendHandler {
	return &EmailSendHandler{
		emailRepo:   emailRepo,
		smtpRepo:    smtpRepo,
		serverRepo:  serverRepo,
		domainRepo:  domainRepo,
		contactRepo: contactRepo,
		sender:      email.NewSMTPSender(),
		dispatcher:  dispatcher,
	}
}

// OnSent sets a callback invoked after each successful email send.
func (h *EmailSendHandler) OnSent(fn func()) { h.onSent = fn }

// OnFailed sets a callback invoked after each permanently failed email send.
func (h *EmailSendHandler) OnFailed(fn func()) { h.onFailed = fn }

// ProcessTask handles an email:send task.
func (h *EmailSendHandler) ProcessTask(_ context.Context, t *asynq.Task) error {
	var payload EmailSendPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	em, err := h.emailRepo.FindByID(payload.EmailID)
	if err != nil {
		return fmt.Errorf("email not found: %w", err)
	}

	// Mark as processing
	em.Status = models.EmailStatusProcessing
	_ = h.emailRepo.Update(em)

	// Resolve sender address for validation/domain matching
	senderAddr := em.Sender
	if parsed, err := mail.ParseAddress(em.Sender); err == nil {
		senderAddr = parsed.Address
	}

	// --- Server selection ---
	// 1. Try the user's own per-account SMTP server first.
	// 2. Fall back to a shared server whose allowed_domains covers the sender domain.
	var smtpServer *models.SMTPServer
	var sharedServerID uint // non-zero when a shared server is used

	userServer, err := h.smtpRepo.FindFirstByUserID(em.UserID)
	if err == nil {
		// Validate the sender against the per-user allowed emails list
		if len(userServer.AllowedEmails) > 0 {
			allowed := false
			for _, e := range userServer.AllowedEmails {
				if e == senderAddr {
					allowed = true
					break
				}
			}
			if !allowed {
				h.markFailed(em, fmt.Sprintf("sender %q is not in the allowed emails list", em.Sender), 0)
				return nil
			}
		}
		smtpServer = userServer
	} else {
		// No per-user server – look for a shared server by sender domain
		if h.serverRepo != nil {
			domain := senderDomain(senderAddr)
			if domain != "" {
				shared, serr := h.serverRepo.FindEnabledByDomain(domain)
				if serr == nil {
					// In strict mode the sender's domain must be ownership-verified.
					if shared.SecurityMode == models.ServerSecurityModeStrict {
						if h.domainRepo == nil || !h.domainRepo.IsOwnershipVerified(em.UserID, domain) {
							h.markFailed(em, fmt.Sprintf("shared server %q requires verified domain ownership for %q", shared.Name, domain), 0)
							return nil
						}
					}
					smtpServer = shared.ToSMTPServer()
					sharedServerID = shared.ID
				}
			}
		}
	}

	if smtpServer == nil {
		h.markFailed(em, "no SMTP server configured for this account or domain", 0)
		// Don't retry – adding a server won't happen automatically.
		return nil
	}

	em.SMTPHostname = smtpServer.Host
	_ = h.emailRepo.Update(em)

	// Auto-generate plain text from HTML if not provided
	if em.TextBody == "" && em.HTMLBody != "" {
		em.TextBody = email.HTMLToText(em.HTMLBody)
	}

	// Parse attachments from stored JSON
	var attachments []models.Attachment
	if em.AttachmentsJSON != "" {
		_ = json.Unmarshal([]byte(em.AttachmentsJSON), &attachments)
	}

	// Parse custom headers from stored JSON
	var headers map[string]string
	if em.HeadersJSON != "" {
		_ = json.Unmarshal([]byte(em.HeadersJSON), &headers)
	}

	if err := h.sender.Send(smtpServer, em.Sender, em.Recipients, em.Subject, em.HTMLBody, em.TextBody, attachments, headers, em.ListUnsubscribeURL, em.ListUnsubscribePost); err != nil {
		em.RetryCount++
		em.Status = models.EmailStatusFailed
		em.ErrorMessage = err.Error()
		_ = h.emailRepo.Update(em)
		// Increment the shared server's failure counter
		if sharedServerID != 0 && h.serverRepo != nil {
			go h.serverRepo.IncrementFailedCount(sharedServerID)
		}
		logger.Debug("worker: email send failed, will retry", "id", em.ID, "attempt", em.RetryCount, "error", err)
		// Return error so Asynq retries the task
		return fmt.Errorf("SMTP send failed: %w", err)
	}

	// Success
	now := time.Now()
	em.Status = models.EmailStatusSent
	em.SentAt = &now
	em.ErrorMessage = ""
	_ = h.emailRepo.Update(em)
	// Increment the shared server's success counter
	if sharedServerID != 0 && h.serverRepo != nil {
		go h.serverRepo.IncrementSentCount(sharedServerID)
	}
	h.dispatcher.Dispatch(em.UserID, "email.sent", em.UUID, em.Sender)
	if h.onSent != nil {
		h.onSent()
	}
	if h.contactRepo != nil {
		go h.contactRepo.RecordSent(em.UserID, em.Recipients)
	}
	logger.Info("worker: email sent successfully", "id", em.ID)

	return nil
}

func (h *EmailSendHandler) markFailed(em *models.Email, reason string, sharedServerID uint) {
	em.Status = models.EmailStatusFailed
	em.ErrorMessage = reason
	_ = h.emailRepo.Update(em)
	if sharedServerID != 0 && h.serverRepo != nil {
		go h.serverRepo.IncrementFailedCount(sharedServerID)
	}
	h.dispatcher.Dispatch(em.UserID, "email.failed", em.UUID, em.Sender)
	if h.onFailed != nil {
		h.onFailed()
	}
	if h.contactRepo != nil {
		go h.contactRepo.RecordFailed(em.UserID, em.Recipients)
	}
}

// senderDomain extracts the domain part of an email address.
func senderDomain(addr string) string {
	parts := strings.SplitN(addr, "@", 2)
	if len(parts) != 2 {
		return ""
	}
	return strings.ToLower(parts[1])
}

// ExhaustedErrorHandler marks emails as permanently failed when Asynq exhausts
// all retries. It implements asynq.ErrorHandler.
type ExhaustedErrorHandler struct {
	emailRepo  *repositories.EmailRepository
	dispatcher *webhook.Dispatcher
	onFailed   func()
}

func NewExhaustedErrorHandler(emailRepo *repositories.EmailRepository, dispatcher *webhook.Dispatcher, onFailed func()) *ExhaustedErrorHandler {
	return &ExhaustedErrorHandler{
		emailRepo:  emailRepo,
		dispatcher: dispatcher,
		onFailed:   onFailed,
	}
}

func (e *ExhaustedErrorHandler) HandleError(_ context.Context, t *asynq.Task, err error) {
	if t.Type() != TypeEmailSend {
		return
	}
	var payload EmailSendPayload
	if jsonErr := json.Unmarshal(t.Payload(), &payload); jsonErr != nil {
		logger.Error("exhausted handler: failed to unmarshal payload", "error", jsonErr)
		return
	}
	em, findErr := e.emailRepo.FindByID(payload.EmailID)
	if findErr != nil {
		return
	}
	em.Status = models.EmailStatusFailed
	em.ErrorMessage = fmt.Sprintf("permanently failed after retries: %v", err)
	_ = e.emailRepo.Update(em)
	e.dispatcher.Dispatch(em.UserID, "email.failed", em.UUID, em.Sender)
	if e.onFailed != nil {
		e.onFailed()
	}
	logger.Error("worker: email permanently failed", "id", em.ID, "error", err)
}
