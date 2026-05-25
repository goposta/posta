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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"strings"
	"time"

	"github.com/goposta/posta/internal/metrics"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/email"
	"github.com/goposta/posta/internal/services/webhook"
	"github.com/goposta/posta/internal/storage/blob"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/hibiken/asynq"
	"github.com/jkaninda/logger"
)

// EmailSendHandler processes email:send tasks from the Asynq queue.
type EmailSendHandler struct {
	emailRepo       *repositories.EmailRepository
	smtpRepo        *repositories.SMTPRepository
	serverRepo      *repositories.ServerRepository
	domainRepo      *repositories.DomainRepository
	contactRepo     *repositories.ContactRepository
	messageRepo     *repositories.CampaignMessageRepository
	campaignRepo    *repositories.CampaignRepository
	suppressionRepo *repositories.SuppressionRepository
	bounceRepo      *repositories.BounceRepository
	autoSuppress    bool
	sender          *email.SMTPSender
	stamper         *email.Stamper
	dispatcher      *webhook.Dispatcher
	blobStore       blob.Store
	onSent          func()
	onFailed        func()
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

// SetBlobStore sets the blob storage backend for fetching attachment content.
func (h *EmailSendHandler) SetBlobStore(bs blob.Store) { h.blobStore = bs }

// SetCampaignMessageRepo sets the campaign message repository so that
// campaign message statuses are updated when emails are sent or fail.
func (h *EmailSendHandler) SetCampaignMessageRepo(r *repositories.CampaignMessageRepository) {
	h.messageRepo = r
}

// SetCampaignRepo lets the handler consult the campaign's live status before
// sending. When set, already-queued emails for cancelled/paused campaigns are
// dropped instead of dispatched.
func (h *EmailSendHandler) SetCampaignRepo(r *repositories.CampaignRepository) {
	h.campaignRepo = r
}

// SetStamper enables X-Mailer/X-Posta-* header stamping and optional
// X-Posta-Signature HMAC. Nil disables stamping.
func (h *EmailSendHandler) SetStamper(s *email.Stamper) {
	h.stamper = s
}

// SetSuppressionRepo enables auto-suppression of recipients permanently
// rejected by the receiving server.
func (h *EmailSendHandler) SetSuppressionRepo(r *repositories.SuppressionRepository) {
	h.suppressionRepo = r
}

// SetBounceRepo lets the handler record a hard bounce when a recipient is
// permanently rejected.
func (h *EmailSendHandler) SetBounceRepo(r *repositories.BounceRepository) {
	h.bounceRepo = r
}

// SetAutoSuppress toggles auto-suppression on permanent (5xx) recipient
// rejection. When false, such failures fall back to the normal retry path.
func (h *EmailSendHandler) SetAutoSuppress(enabled bool) {
	h.autoSuppress = enabled
}

// OnSent sets a callback invoked after each successful email send.
func (h *EmailSendHandler) OnSent(fn func()) { h.onSent = fn }

// OnFailed sets a callback invoked after each permanently failed email send.
func (h *EmailSendHandler) OnFailed(fn func()) { h.onFailed = fn }

// ProcessTask handles an email:send task.
func (h *EmailSendHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload EmailSendPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	em, err := h.emailRepo.FindByID(payload.EmailID)
	if err != nil {
		return fmt.Errorf("email not found: %w", err)
	}

	// If this email was enqueued as part of a campaign, respect the live
	// campaign status. A paused campaign releases the message back to pending
	// so a Resume will re-dispatch it; cancelled drops it permanently.
	if h.messageRepo != nil && h.campaignRepo != nil {
		if msg, mErr := h.messageRepo.FindByEmailID(em.ID); mErr == nil {
			if camp, cErr := h.campaignRepo.FindByID(msg.CampaignID); cErr == nil {
				switch camp.Status {
				case models.CampaignStatusCancelled:
					_ = h.messageRepo.UpdateStatus(msg.ID, models.CampaignMsgSkipped, "campaign cancelled")
					em.Status = models.EmailStatusFailed
					em.ErrorMessage = "campaign cancelled"
					_ = h.emailRepo.Update(em)
					logger.Info("worker: dropping email for cancelled campaign", "email_id", em.ID, "campaign_id", camp.ID)
					return nil
				case models.CampaignStatusPaused:
					_ = h.messageRepo.UpdateStatus(msg.ID, models.CampaignMsgPending, "")
					em.Status = models.EmailStatusQueued
					_ = h.emailRepo.Update(em)
					logger.Info("worker: releasing email back to pending (campaign paused)", "email_id", em.ID, "campaign_id", camp.ID)
					return nil
				}
			}
		}
	}

	// Mark as processing
	em.Status = models.EmailStatusProcessing
	_ = h.emailRepo.Update(em)

	// Resolve sender address for validation/domain matching
	senderAddr := em.Sender
	if parsed, err := mail.ParseAddress(em.Sender); err == nil {
		senderAddr = parsed.Address
	}

	// Server selection:
	// 1. Try the workspace or user's own per-account SMTP server first.
	// 2. Fall back to a shared server whose allowed_domains covers the sender domain.
	var smtpServer *models.SMTPServer
	var sharedServerID uint // non-zero when a shared server is used

	var userServer *models.SMTPServer
	if em.WorkspaceID != nil {
		userServer, err = h.smtpRepo.FindFirstByWorkspaceID(*em.WorkspaceID)
	} else {
		userServer, err = h.smtpRepo.FindFirstByUserID(em.UserID)
	}
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
				h.markFailed(em, fmt.Sprintf("sender %q is not in the allowed emails list", em.Sender))
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
							h.markFailed(em, fmt.Sprintf("shared server %q requires verified domain ownership for %q", shared.Name, domain))
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
		h.markFailed(em, "no SMTP server configured for this account or domain")
		// Don't retry – adding a server won't happen automatically.
		return nil
	}

	em.SMTPHostname = smtpServer.Host
	_ = h.emailRepo.Update(em)

	// Auto-generate plain text from HTML if not provided
	if em.TextBody == "" && em.HTMLBody != "" {
		em.TextBody = email.HTMLToText(em.HTMLBody)
	}

	// Parse attachments from stored JSON and fetch content from blob storage if needed
	var attachments []models.Attachment
	if em.AttachmentsJSON != "" {
		_ = json.Unmarshal([]byte(em.AttachmentsJSON), &attachments)
		if h.blobStore != nil {
			for i, att := range attachments {
				if att.StorageKey != "" && att.Content == "" {
					rc, err := h.blobStore.Get(ctx, att.StorageKey)
					if err != nil {
						h.markFailed(em, fmt.Sprintf("failed to fetch attachment %q from storage: %v", att.Filename, err))
						return nil
					}
					data, err := io.ReadAll(rc)
					_ = rc.Close()
					if err != nil {
						h.markFailed(em, fmt.Sprintf("failed to read attachment %q: %v", att.Filename, err))
						return nil
					}
					attachments[i].Content = base64.StdEncoding.EncodeToString(data)
				}
			}
		}
	}

	// Parse custom headers from stored JSON
	var headers map[string]string
	if em.HeadersJSON != "" {
		_ = json.Unmarshal([]byte(em.HeadersJSON), &headers)
	}
	if headers == nil {
		headers = make(map[string]string)
	}

	// Drop recipients that became suppressed since this email was enqueued
	recipients := []string(em.Recipients)
	if h.suppressionRepo != nil {
		scope := repositories.ResourceScope{UserID: em.UserID, WorkspaceID: em.WorkspaceID}
		if filtered, fErr := h.suppressionRepo.FilterSuppressed(scope, recipients); fErr == nil {
			if len(filtered) == 0 {
				em.Status = models.EmailStatusSuppressed
				em.ErrorMessage = "all recipients are suppressed"
				_ = h.emailRepo.Update(em)
				if h.messageRepo != nil {
					if msg, mErr := h.messageRepo.FindByEmailID(em.ID); mErr == nil {
						_ = h.messageRepo.UpdateStatus(msg.ID, models.CampaignMsgSkipped, "recipient suppressed")
					}
				}
				logger.Info("worker: skipping email, all recipients suppressed", "id", em.ID)
				return nil
			}
			recipients = filtered
		}
	}

	// Stamp Posta headers. Campaign mail gets campaign-aware headers; everything
	// else gets transactional headers. A final HMAC signs the message identity.
	if h.stamper != nil {
		var campMsg *models.CampaignMessage
		if h.messageRepo != nil {
			if m, err := h.messageRepo.FindByEmailID(em.ID); err == nil {
				campMsg = m
			}
		}
		if campMsg != nil {
			// Opens and clicks are rewritten together in the tracking service's
			tracked := em.HTMLBody != "" && em.ListUnsubscribeURL != ""
			h.stamper.StampCampaign(
				headers, em,
				campMsg.CampaignID, campMsg.ID,
				tracked, tracked,
			)
		} else {
			h.stamper.StampTransactional(headers, em)
		}
		h.stamper.Sign(headers, em, recipients, em.Subject)
	}

	if err := h.sender.Send(smtpServer, em.Sender, recipients, em.Subject, em.HTMLBody, em.TextBody, attachments, headers, em.ListUnsubscribeURL, em.ListUnsubscribePost); err != nil {
		// Increment the shared server's failure counter regardless of outcome.
		if sharedServerID != 0 && h.serverRepo != nil {
			go h.serverRepo.IncrementFailedCount(sharedServerID)
		}

		// A permanent rejection at RCPT TO (5xx, e.g. 550 user unknown) is a
		// hard bounce: suppress the recipient and stop retrying — retrying a
		// 5xx is pointless.
		if se, ok := permanentRejection(err); ok && h.autoSuppress {
			h.handlePermanentRejection(em, se)
			logger.Info("worker: recipient permanently rejected, suppressed", "id", em.ID, "recipient", se.Recipient, "code", se.Code)
			return nil
		}

		em.RetryCount++
		em.Status = models.EmailStatusFailed
		em.ErrorMessage = err.Error()
		_ = h.emailRepo.Update(em)
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
	h.dispatcher.Dispatch(em.UserID, em.WorkspaceID, "email.sent", em.UUID, em.Sender)
	if h.onSent != nil {
		h.onSent()
	}
	if h.contactRepo != nil {
		go h.contactRepo.RecordSent(em.UserID, em.WorkspaceID, recipients)
	}
	if h.messageRepo != nil {
		if msg, err := h.messageRepo.FindByEmailID(em.ID); err == nil {
			_ = h.messageRepo.UpdateStatus(msg.ID, models.CampaignMsgSent, "")
		}
	}
	logger.Info("worker: email sent successfully", "id", em.ID)

	return nil
}

func (h *EmailSendHandler) markFailed(em *models.Email, reason string) {
	em.Status = models.EmailStatusFailed
	em.ErrorMessage = reason
	_ = h.emailRepo.Update(em)
	h.dispatcher.Dispatch(em.UserID, em.WorkspaceID, "email.failed", em.UUID, em.Sender)
	if h.onFailed != nil {
		h.onFailed()
	}
	if h.contactRepo != nil {
		go h.contactRepo.RecordFailed(em.UserID, em.WorkspaceID, em.Recipients)
	}
	if h.messageRepo != nil {
		if msg, err := h.messageRepo.FindByEmailID(em.ID); err == nil {
			_ = h.messageRepo.UpdateStatus(msg.ID, models.CampaignMsgFailed, reason)
		}
	}
}

// permanentRejection reports whether err represents a permanent recipient
// rejection (a 5xx reply at RCPT TO, e.g. 550 user unknown) that should trigger
// auto-suppression. Transient errors (4xx), connection failures, and failures
// at other SMTP stages (MAIL FROM, DATA) return false so they keep retrying.
func permanentRejection(err error) (*email.SendError, bool) {
	var se *email.SendError
	if errors.As(err, &se) && se.Permanent() && se.Stage == "RCPT TO" && se.Recipient != "" {
		return se, true
	}
	return nil, false
}

// handlePermanentRejection records a hard bounce, adds the recipient to the
// suppression list (idempotently) and marks the email failed without retrying.
// Called when the receiving server permanently rejects a recipient (5xx at
// RCPT TO, e.g. 550 user unknown).
func (h *EmailSendHandler) handlePermanentRejection(em *models.Email, se *email.SendError) {
	reason := se.Msg
	if reason == "" {
		reason = se.Error()
	}

	// Record a hard bounce for audit/metrics.
	if h.bounceRepo != nil {
		bounce := &models.Bounce{
			UserID:      em.UserID,
			WorkspaceID: em.WorkspaceID,
			EmailID:     em.ID,
			Recipient:   se.Recipient,
			Type:        models.BounceTypeHard,
			Reason:      reason,
		}
		if err := h.bounceRepo.Create(bounce); err == nil {
			metrics.IncrementBounce(string(models.BounceTypeHard))
		}
	}

	// Add to the suppression list (idempotent — ignore "already suppressed").
	if h.suppressionRepo != nil {
		suppression := &models.Suppression{
			UserID:      em.UserID,
			WorkspaceID: em.WorkspaceID,
			Email:       se.Recipient,
			Reason:      fmt.Sprintf("auto-suppressed: SMTP %d %s", se.Code, reason),
		}
		if err := h.suppressionRepo.Upsert(suppression); err == nil {
			metrics.IncrementSuppression()
		}
	}

	em.Status = models.EmailStatusFailed
	em.ErrorMessage = se.Error()
	_ = h.emailRepo.Update(em)

	h.dispatcher.Dispatch(em.UserID, em.WorkspaceID, "email.failed", em.UUID, em.Sender)
	if h.onFailed != nil {
		h.onFailed()
	}
	if h.contactRepo != nil {
		go h.contactRepo.RecordFailed(em.UserID, em.WorkspaceID, em.Recipients)
	}
	if h.messageRepo != nil {
		if msg, err := h.messageRepo.FindByEmailID(em.ID); err == nil {
			_ = h.messageRepo.UpdateStatus(msg.ID, models.CampaignMsgFailed, reason)
		}
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
	e.dispatcher.Dispatch(em.UserID, em.WorkspaceID, "email.failed", em.UUID, em.Sender)
	if e.onFailed != nil {
		e.onFailed()
	}
	logger.Error("worker: email permanently failed", "id", em.ID, "error", err)
}
