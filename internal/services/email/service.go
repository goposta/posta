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

package email

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/mail"
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/ratelimit"
	"github.com/goposta/posta/internal/services/settings"
	"github.com/goposta/posta/internal/services/webhook"
	"github.com/goposta/posta/internal/storage/blob"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/logger"
)

const (
	DefaultMaxAttachmentSize int64 = 10 * 1024 * 1024 // 10MB per attachment
	DefaultMaxTotalSize      int64 = 25 * 1024 * 1024 // 25MB total

	defaultFromAddress = "noreply@localhost"
)

// EmailEnqueuer enqueues an email for background delivery.
// When set on the Service, emails are sent asynchronously via a worker.
type EmailEnqueuer interface {
	EnqueueEmailSend(emailID uint, queue string) error
	EnqueueEmailSendAt(emailID uint, queue string, sendAt time.Time) error
}

// PlanLimitsProvider resolves effective plan limits for a given workspace.
type PlanLimitsProvider interface {
	EffectiveLimits(workspaceID *uint) *PlanLimits
}

// PlanLimits holds the resolved limits from a plan or global settings.
type PlanLimits struct {
	HourlyRateLimit     int
	DailyRateLimit      int
	MaxAttachmentSizeMB int
	MaxBatchSize        int
}

type Service struct {
	emailRepo        *repositories.EmailRepository
	smtpRepo         *repositories.SMTPRepository
	templateRepo     *repositories.TemplateRepository
	suppressionRepo  *repositories.SuppressionRepository
	versionRepo      *repositories.TemplateVersionRepository
	localizationRepo *repositories.TemplateLocalizationRepository
	contactRepo      *repositories.ContactRepository
	domainRepo       *repositories.DomainRepository
	userRepo         *repositories.UserRepository
	sender           *SMTPSender
	renderer         *TemplateRenderer
	limiter          *ratelimit.RedisLimiter
	dispatcher       *webhook.Dispatcher
	enqueuer         EmailEnqueuer
	settings         *settings.Provider
	blobStore        blob.Store
	planLimits       PlanLimitsProvider
	devMode          bool
	onSent           func()
	onFailed         func()
	onQueued         func()
}

func NewService(
	emailRepo *repositories.EmailRepository,
	smtpRepo *repositories.SMTPRepository,
	templateRepo *repositories.TemplateRepository,
	suppressionRepo *repositories.SuppressionRepository,
	limiter *ratelimit.RedisLimiter,
	dispatcher *webhook.Dispatcher,
	devMode bool,
) *Service {
	return &Service{
		emailRepo:       emailRepo,
		smtpRepo:        smtpRepo,
		templateRepo:    templateRepo,
		suppressionRepo: suppressionRepo,
		sender:          NewSMTPSender(),
		renderer:        NewTemplateRenderer(),
		limiter:         limiter,
		dispatcher:      dispatcher,
		devMode:         devMode,
	}
}

// SetVersionRepos sets the version and localization repositories for multi-language support.
func (s *Service) SetVersionRepos(vr *repositories.TemplateVersionRepository, lr *repositories.TemplateLocalizationRepository) {
	s.versionRepo = vr
	s.localizationRepo = lr
}

// SetContactRepo sets the contact repository for tracking recipient stats.
func (s *Service) SetContactRepo(cr *repositories.ContactRepository) {
	s.contactRepo = cr
}

// SetDomainVerification sets the domain and user repositories for enforcing verified domain sending.
func (s *Service) SetDomainVerification(dr *repositories.DomainRepository, ur *repositories.UserRepository) {
	s.domainRepo = dr
	s.userRepo = ur
}

// SetSettings sets the platform settings provider for dynamic configuration.
func (s *Service) SetSettings(sp *settings.Provider) {
	s.settings = sp
}

// SetPlanLimits sets the plan limits provider for plan-aware enforcement.
func (s *Service) SetPlanLimits(pl PlanLimitsProvider) {
	s.planLimits = pl
}

// SetEnqueuer sets the email enqueuer for asynchronous delivery.
// When set, emails are enqueued to a background worker instead of being sent synchronously.
func (s *Service) SetEnqueuer(eq EmailEnqueuer) {
	s.enqueuer = eq
}

// SetBlobStore sets the blob storage backend for persisting email attachments.
// When configured, attachment content is uploaded to the store and only metadata
// (with a storage key) is kept in the database, reducing DB pressure.
func (s *Service) SetBlobStore(bs blob.Store) {
	s.blobStore = bs
}

// uploadAttachments stores each attachment's content in blob storage and replaces
// the inline base64 content with a storage key reference. Returns the modified
// attachments with StorageKey set and Content cleared.
func (s *Service) uploadAttachments(ctx context.Context, emailUUID string, attachments []models.Attachment) ([]models.Attachment, error) {
	result := make([]models.Attachment, len(attachments))
	for i, att := range attachments {
		decoded, err := base64.StdEncoding.DecodeString(att.Content)
		if err != nil {
			return nil, fmt.Errorf("attachment %q: invalid base64: %w", att.Filename, err)
		}
		key := fmt.Sprintf("emails/%s/%d_%s", emailUUID, i, att.Filename)
		if err := s.blobStore.Put(ctx, key, bytes.NewReader(decoded), att.ContentType); err != nil {
			return nil, fmt.Errorf("failed to upload attachment %q: %w", att.Filename, err)
		}
		result[i] = models.Attachment{
			Filename:    att.Filename,
			ContentType: att.ContentType,
			StorageKey:  key,
		}
	}
	return result, nil
}

// DownloadAttachments fetches attachment content from blob storage and populates
// the base64 Content field for each attachment that has a StorageKey.
func (s *Service) DownloadAttachments(ctx context.Context, attachments []models.Attachment) ([]models.Attachment, error) {
	result := make([]models.Attachment, len(attachments))
	for i, att := range attachments {
		result[i] = att
		if att.StorageKey == "" {
			continue
		}
		rc, err := s.blobStore.Get(ctx, att.StorageKey)
		if err != nil {
			return nil, fmt.Errorf("failed to download attachment %q: %w", att.Filename, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read attachment %q: %w", att.Filename, err)
		}
		result[i].Content = base64.StdEncoding.EncodeToString(data)
	}
	return result, nil
}

// DeleteAttachments removes attachment blobs from storage for the given keys.
func (s *Service) DeleteAttachments(ctx context.Context, attachments []models.Attachment) {
	for _, att := range attachments {
		if att.StorageKey != "" {
			_ = s.blobStore.Delete(ctx, att.StorageKey)
		}
	}
}

// OnSent sets a callback invoked after each successful email send.
func (s *Service) OnSent(fn func()) { s.onSent = fn }

// OnFailed sets a callback invoked after each failed email send.
func (s *Service) OnFailed(fn func()) { s.onFailed = fn }

// OnQueued sets a callback invoked after each email is enqueued for delivery.
func (s *Service) OnQueued(fn func()) { s.onQueued = fn }

type SendRequest struct {
	From                string              `json:"from" required:"true"`
	To                  []string            `json:"to" required:"true" minItems:"1"`
	Subject             string              `json:"subject" required:"true"`
	HTML                string              `json:"html"`
	Text                string              `json:"text"`
	Attachments         []models.Attachment `json:"attachments,omitempty"`
	Headers             map[string]string   `json:"headers,omitempty"`
	ListUnsubscribeURL  string              `json:"list_unsubscribe_url,omitempty"`
	ListUnsubscribePost bool                `json:"list_unsubscribe_post,omitempty"`
	SendAt              *time.Time          `json:"send_at,omitempty"`
}

type SendTemplateRequest struct {
	TemplateID   *uint               `json:"template_id,omitempty" doc:"Template ID (preferred, uses primary key index)"`
	Template     string              `json:"template" doc:"Template name (fallback when template_id is not provided)"`
	Language     string              `json:"language"`
	From         string              `json:"from"`
	To           []string            `json:"to" required:"true" minItems:"1"`
	TemplateData map[string]any      `json:"template_data"`
	Attachments  []models.Attachment `json:"attachments,omitempty"`
}

type SendResponse struct {
	ID     string             `json:"id"`
	Status models.EmailStatus `json:"status"`
}

type BatchRequest struct {
	TemplateID *uint            `json:"template_id,omitempty" doc:"Template ID (preferred, uses primary key index)"`
	Template   string           `json:"template" doc:"Template name (fallback when template_id is not provided)"`
	Language   string           `json:"language"`
	From       string           `json:"from"`
	Recipients []BatchRecipient `json:"recipients" required:"true" minItems:"1"`
}

type BatchRecipient struct {
	Email        string         `json:"email" required:"true" format:"email"`
	Language     string         `json:"language"`
	TemplateData map[string]any `json:"template_data"`
}

type BatchResponse struct {
	Total   int           `json:"total"`
	Sent    int           `json:"sent"`
	Failed  int           `json:"failed"`
	Skipped int           `json:"skipped"`
	Results []BatchResult `json:"results"`
}

type BatchResult struct {
	Email  string             `json:"email"`
	ID     string             `json:"id,omitempty"`
	Status models.EmailStatus `json:"status"`
	Error  string             `json:"error,omitempty"`
}

// DryRunResponse is returned when a send request is validated without actually sending.
type DryRunResponse struct {
	Valid           bool     `json:"valid"`
	From            string   `json:"from"`
	To              []string `json:"to"`
	Subject         string   `json:"subject"`
	Recipients      int      `json:"recipients"`
	SuppressedCount int      `json:"suppressed_count"`
	AttachmentCount int      `json:"attachment_count"`
	HasHTML         bool     `json:"has_html"`
	HasText         bool     `json:"has_text"`
	RenderedSubject string   `json:"rendered_subject,omitempty"`
}

// ValidateSend runs all validation checks without persisting or sending.
func (s *Service) ValidateSend(ctx context.Context, userID uint, workspaceID *uint, userEmail string, req *SendRequest) (*DryRunResponse, error) {
	if err := validateAddresses(req.From, req.To); err != nil {
		return nil, err
	}

	if s.settings != nil && s.settings.MaintenanceMode() {
		return nil, fmt.Errorf("maintenance: email sending is temporarily disabled")
	}

	if err := s.limiter.Check(ctx, userEmail); err != nil {
		return nil, fmt.Errorf("rate_limit: %w", err)
	}

	if len(req.Attachments) > 0 {
		maxSize := DefaultMaxAttachmentSize
		if s.planLimits != nil {
			pl := s.planLimits.EffectiveLimits(workspaceID)
			if pl.MaxAttachmentSizeMB > 0 {
				maxSize = int64(pl.MaxAttachmentSizeMB) * 1024 * 1024
			}
		} else if s.settings != nil {
			maxSize = int64(s.settings.MaxAttachmentSizeMB()) * 1024 * 1024
		}
		if err := ValidateAttachments(req.Attachments, maxSize, DefaultMaxTotalSize); err != nil {
			return nil, fmt.Errorf("attachment validation: %w", err)
		}
	}

	if err := s.checkDomainVerification(userID, req.From); err != nil {
		return nil, err
	}

	scope := repositories.ResourceScope{UserID: userID, WorkspaceID: workspaceID}
	activeRecipients := req.To
	suppressedCount := 0
	if s.suppressionRepo != nil {
		filtered, err := s.suppressionRepo.FilterSuppressed(scope, req.To)
		if err != nil {
			return nil, fmt.Errorf("failed to check suppression list: %w", err)
		}
		suppressedCount = len(req.To) - len(filtered)
		activeRecipients = filtered
	}

	return &DryRunResponse{
		Valid:           true,
		From:            req.From,
		To:              activeRecipients,
		Subject:         req.Subject,
		Recipients:      len(activeRecipients),
		SuppressedCount: suppressedCount,
		AttachmentCount: len(req.Attachments),
		HasHTML:         req.HTML != "",
		HasText:         req.Text != "",
	}, nil
}

// ValidateSendWithTemplate validates a template send request without sending.
func (s *Service) ValidateSendWithTemplate(ctx context.Context, userID uint, workspaceID *uint, userEmail string, req *SendTemplateRequest) (*DryRunResponse, error) {
	tmpl, err := s.findTemplate(userID, workspaceID, req.TemplateID, req.Template)
	if err != nil {
		return nil, fmt.Errorf("template not found: %s", req.Template)
	}

	rendered, err := s.resolveAndRender(tmpl, req.Language, req.TemplateData)
	if err != nil {
		return nil, fmt.Errorf("failed to render template: %w", err)
	}

	from := req.From
	if from == "" {
		from = defaultFromAddress
	}

	resp, err := s.ValidateSend(ctx, userID, workspaceID, userEmail, &SendRequest{
		From:        from,
		To:          req.To,
		Subject:     rendered.Subject,
		HTML:        rendered.HTML,
		Text:        rendered.Text,
		Attachments: req.Attachments,
	})
	if err != nil {
		return nil, err
	}
	resp.RenderedSubject = rendered.Subject
	return resp, nil
}

// ValidateSendBatch validates a batch send request without sending.
func (s *Service) ValidateSendBatch(ctx context.Context, userID uint, workspaceID *uint, userEmail string, req *BatchRequest) (*DryRunResponse, error) {
	from := req.From
	if from == "" {
		from = defaultFromAddress
	}
	if err := validateEmailAddress(from); err != nil {
		return nil, fmt.Errorf("invalid sender: %w", err)
	}
	for _, r := range req.Recipients {
		if err := validateEmailAddress(r.Email); err != nil {
			return nil, fmt.Errorf("invalid recipient: %w", err)
		}
	}

	maxBatch := 0
	if s.planLimits != nil {
		maxBatch = s.planLimits.EffectiveLimits(workspaceID).MaxBatchSize
	} else if s.settings != nil {
		maxBatch = s.settings.MaxBatchSize()
	}
	if maxBatch > 0 && len(req.Recipients) > maxBatch {
		return nil, fmt.Errorf("batch size %d exceeds maximum allowed (%d)", len(req.Recipients), maxBatch)
	}

	_, err := s.findTemplate(userID, workspaceID, req.TemplateID, req.Template)
	if err != nil {
		return nil, fmt.Errorf("template not found: %w", err)
	}

	emails := make([]string, len(req.Recipients))
	for i, r := range req.Recipients {
		emails[i] = r.Email
	}

	scope := repositories.ResourceScope{UserID: userID, WorkspaceID: workspaceID}
	activeRecipients := emails
	suppressedCount := 0
	if s.suppressionRepo != nil {
		filtered, err := s.suppressionRepo.FilterSuppressed(scope, emails)
		if err != nil {
			return nil, fmt.Errorf("failed to check suppression list: %w", err)
		}
		suppressedCount = len(emails) - len(filtered)
		activeRecipients = filtered
	}

	return &DryRunResponse{
		Valid:           true,
		From:            from,
		To:              activeRecipients,
		Subject:         req.Template,
		Recipients:      len(activeRecipients),
		SuppressedCount: suppressedCount,
	}, nil
}

func (s *Service) Send(ctx context.Context, userID, apiKeyID uint, workspaceID *uint, userEmail string, req *SendRequest) (*SendResponse, error) {
	// Validate email addresses
	if err := validateAddresses(req.From, req.To); err != nil {
		return nil, err
	}

	// Enforce maintenance mode
	if s.settings != nil && s.settings.MaintenanceMode() {
		return nil, fmt.Errorf("maintenance: email sending is temporarily disabled")
	}

	// Rate limiting: use plan limits if available, otherwise global.
	if s.planLimits != nil {
		pl := s.planLimits.EffectiveLimits(workspaceID)
		if err := s.limiter.AllowWithLimits(ctx, userEmail, pl.HourlyRateLimit, pl.DailyRateLimit); err != nil {
			return nil, fmt.Errorf("rate_limit: %w", err)
		}
	} else {
		if err := s.limiter.Allow(ctx, userEmail); err != nil {
			return nil, fmt.Errorf("rate_limit: %w", err)
		}
	}

	// Validate attachments if present
	if len(req.Attachments) > 0 {
		maxSize := DefaultMaxAttachmentSize
		if s.planLimits != nil {
			pl := s.planLimits.EffectiveLimits(workspaceID)
			if pl.MaxAttachmentSizeMB > 0 {
				maxSize = int64(pl.MaxAttachmentSizeMB) * 1024 * 1024
			}
		} else if s.settings != nil {
			maxSize = int64(s.settings.MaxAttachmentSizeMB()) * 1024 * 1024
		}
		if err := ValidateAttachments(req.Attachments, maxSize, DefaultMaxTotalSize); err != nil {
			return nil, fmt.Errorf("attachment validation: %w", err)
		}
	}

	// Enforce verified domain when user has strict mode enabled (personal mode only;
	// workspace domain verification is handled at the workspace level).
	if workspaceID == nil {
		if err := s.checkDomainVerification(userID, req.From); err != nil {
			return nil, err
		}
	}

	// Filter out suppressed recipients
	scope := repositories.ResourceScope{UserID: userID, WorkspaceID: workspaceID}
	if s.suppressionRepo != nil {
		filtered, err := s.suppressionRepo.FilterSuppressed(scope, req.To)
		if err != nil {
			return nil, fmt.Errorf("failed to check suppression list: %w", err)
		}
		if len(filtered) == 0 {
			// Log the suppressed email attempt
			var apiKeyPtr *uint
			if apiKeyID > 0 {
				apiKeyPtr = &apiKeyID
			}
			em := &models.Email{
				UserID:       userID,
				WorkspaceID:  workspaceID,
				APIKeyID:     apiKeyPtr,
				Sender:       req.From,
				Recipients:   req.To,
				Subject:      req.Subject,
				HTMLBody:     req.HTML,
				TextBody:     req.Text,
				Status:       models.EmailStatusSuppressed,
				ErrorMessage: "all recipients are suppressed",
			}
			_ = s.emailRepo.Create(em)
			return &SendResponse{ID: em.UUID, Status: em.Status}, nil
		}
		// Log suppressed recipients individually if some were filtered out
		if len(filtered) < len(req.To) {
			suppressed := diffRecipients(req.To, filtered)
			var apiKeyPtr *uint
			if apiKeyID > 0 {
				apiKeyPtr = &apiKeyID
			}
			for _, addr := range suppressed {
				em := &models.Email{
					UserID:       userID,
					WorkspaceID:  workspaceID,
					APIKeyID:     apiKeyPtr,
					Sender:       req.From,
					Recipients:   []string{addr},
					Subject:      req.Subject,
					Status:       models.EmailStatusSuppressed,
					ErrorMessage: "recipient is suppressed",
				}
				_ = s.emailRepo.Create(em)
			}
		}
		req.To = filtered
	}

	// Serialize attachments for storage. When blob storage is configured,
	// upload the binary content and store only the storage key reference.
	// Otherwise store just filename + content_type metadata.
	var attachmentsJSON string
	if len(req.Attachments) > 0 {
		if s.blobStore != nil {
			// Generate a temporary UUID for the blob key prefix.
			// The real email UUID is assigned by the DB, so we use a timestamp-based key.
			tempKey := fmt.Sprintf("%d", time.Now().UnixNano())
			uploaded, err := s.uploadAttachments(ctx, tempKey, req.Attachments)
			if err != nil {
				return nil, fmt.Errorf("attachment upload: %w", err)
			}
			b, _ := json.Marshal(uploaded)
			attachmentsJSON = string(b)
		} else {
			type attachMeta struct {
				Filename    string `json:"filename"`
				ContentType string `json:"content_type"`
			}
			meta := make([]attachMeta, len(req.Attachments))
			for i, a := range req.Attachments {
				meta[i] = attachMeta{Filename: a.Filename, ContentType: a.ContentType}
			}
			b, _ := json.Marshal(meta)
			attachmentsJSON = string(b)
		}
	}

	// Auto-generate plain text from HTML when text body is not provided
	if req.Text == "" && req.HTML != "" {
		req.Text = HTMLToText(req.HTML)
	}

	// Serialize custom headers for storage
	var headersJSON string
	if len(req.Headers) > 0 {
		filtered := filterCustomHeaders(req.Headers)
		if len(filtered) > 0 {
			b, _ := json.Marshal(filtered)
			headersJSON = string(b)
		}
	}

	var apiKeyPtr *uint
	if apiKeyID > 0 {
		apiKeyPtr = &apiKeyID
	}

	em := &models.Email{
		UserID:              userID,
		WorkspaceID:         workspaceID,
		APIKeyID:            apiKeyPtr,
		Sender:              req.From,
		Recipients:          req.To,
		Subject:             req.Subject,
		HTMLBody:            req.HTML,
		TextBody:            req.Text,
		AttachmentsJSON:     attachmentsJSON,
		HeadersJSON:         headersJSON,
		ListUnsubscribeURL:  req.ListUnsubscribeURL,
		ListUnsubscribePost: req.ListUnsubscribePost,
		Status:              models.EmailStatusPending,
	}

	if err := s.emailRepo.Create(em); err != nil {
		return nil, fmt.Errorf("failed to store email: %w", err)
	}

	if s.devMode {
		em.Status = models.EmailStatusSent
		now := time.Now()
		em.SentAt = &now
		_ = s.emailRepo.Update(em)
		if s.onSent != nil {
			s.onSent()
		}
		logger.Info("dev mode: email stored but not sent", "id", em.UUID)
		return &SendResponse{ID: em.UUID, Status: em.Status}, nil
	}

	// Scheduled sending
	if req.SendAt != nil && req.SendAt.After(time.Now()) && s.enqueuer != nil {
		em.Status = models.EmailStatusScheduled
		em.ScheduledAt = req.SendAt
		_ = s.emailRepo.Update(em)
		if err := s.enqueuer.EnqueueEmailSendAt(em.ID, "", *req.SendAt); err != nil {
			em.Status = models.EmailStatusFailed
			em.ErrorMessage = fmt.Sprintf("failed to schedule: %v", err)
			_ = s.emailRepo.Update(em)
			return nil, fmt.Errorf("failed to schedule email: %w", err)
		}
		return &SendResponse{ID: em.UUID, Status: em.Status}, nil
	}

	// Asynchronous path: enqueue the email for background delivery
	if s.enqueuer != nil {
		em.Status = models.EmailStatusQueued
		_ = s.emailRepo.Update(em)
		if err := s.enqueuer.EnqueueEmailSend(em.ID, ""); err != nil {
			em.Status = models.EmailStatusFailed
			em.ErrorMessage = fmt.Sprintf("failed to enqueue: %v", err)
			_ = s.emailRepo.Update(em)
			if s.onFailed != nil {
				s.onFailed()
			}
			return nil, fmt.Errorf("failed to enqueue email: %w", err)
		}
		if s.onQueued != nil {
			s.onQueued()
		}
		logger.Debug("email enqueued for background delivery", "id", em.UUID)
		return &SendResponse{ID: em.UUID, Status: em.Status}, nil
	}

	// Synchronous fallback: send email directly (no worker configured)
	return s.sendSync(em, userID, workspaceID, req)
}

// sendSync performs synchronous SMTP delivery (used when no enqueuer is configured).
func (s *Service) sendSync(em *models.Email, userID uint, workspaceID *uint, req *SendRequest) (*SendResponse, error) {
	var smtpServer *models.SMTPServer
	var err error
	if workspaceID != nil {
		smtpServer, err = s.smtpRepo.FindFirstByWorkspaceID(*workspaceID)
	} else {
		smtpServer, err = s.smtpRepo.FindFirstByUserID(userID)
	}
	if err != nil {
		em.Status = models.EmailStatusFailed
		em.ErrorMessage = "no SMTP server configured"
		_ = s.emailRepo.Update(em)
		s.dispatcher.Dispatch(userID, "email.failed", em.UUID, req.From)
		if s.onFailed != nil {
			s.onFailed()
		}
		return nil, fmt.Errorf("no SMTP server configured")
	}

	if len(smtpServer.AllowedEmails) > 0 {
		bareFrom := extractEmail(req.From)
		allowed := false
		for _, e := range smtpServer.AllowedEmails {
			if e == bareFrom {
				allowed = true
				break
			}
		}
		if !allowed {
			em.Status = models.EmailStatusFailed
			em.ErrorMessage = fmt.Sprintf("sender %q is not in the allowed emails list", req.From)
			_ = s.emailRepo.Update(em)
			s.dispatcher.Dispatch(userID, "email.failed", em.UUID, req.From)
			if s.onFailed != nil {
				s.onFailed()
			}
			return nil, fmt.Errorf("sender %q is not in the allowed emails list", req.From)
		}
	}

	em.SMTPHostname = smtpServer.Host
	_ = s.emailRepo.Update(em)

	if err := s.sender.Send(smtpServer, req.From, req.To, req.Subject, req.HTML, req.Text, req.Attachments, req.Headers, req.ListUnsubscribeURL, req.ListUnsubscribePost); err != nil {
		em.Status = models.EmailStatusFailed
		em.ErrorMessage = err.Error()
		_ = s.emailRepo.Update(em)
		s.dispatcher.Dispatch(userID, "email.failed", em.UUID, req.From)
		if s.onFailed != nil {
			s.onFailed()
		}
		if s.contactRepo != nil {
			go s.contactRepo.RecordFailed(userID, workspaceID, req.To)
		}
		return nil, fmt.Errorf("failed to send email: %w", err)
	}

	now := time.Now()
	em.Status = models.EmailStatusSent
	em.SentAt = &now
	_ = s.emailRepo.Update(em)
	s.dispatcher.Dispatch(userID, "email.sent", em.UUID, req.From)
	if s.onSent != nil {
		s.onSent()
	}
	if s.contactRepo != nil {
		go s.contactRepo.RecordSent(userID, workspaceID, req.To)
	}

	return &SendResponse{ID: em.UUID, Status: em.Status}, nil
}

func (s *Service) SendWithTemplate(ctx context.Context, userID, apiKeyID uint, workspaceID *uint, userEmail string, req *SendTemplateRequest) (*SendResponse, error) {
	tmpl, err := s.findTemplate(userID, workspaceID, req.TemplateID, req.Template)
	if err != nil {
		return nil, fmt.Errorf("template not found: %w", err)
	}

	rendered, err := s.resolveAndRender(tmpl, req.Language, req.TemplateData)
	if err != nil {
		return nil, fmt.Errorf("failed to render template: %w", err)
	}

	from := req.From
	if from == "" {
		from = defaultFromAddress
	}

	return s.Send(ctx, userID, apiKeyID, workspaceID, userEmail, &SendRequest{
		From:        from,
		To:          req.To,
		Subject:     rendered.Subject,
		HTML:        rendered.HTML,
		Text:        rendered.Text,
		Attachments: req.Attachments,
	})
}

func (s *Service) SendBatch(ctx context.Context, userID, apiKeyID uint, workspaceID *uint, userEmail string, req *BatchRequest) (*BatchResponse, error) {
	// Enforce max batch size from plan or platform settings
	maxBatch := 0
	if s.planLimits != nil {
		maxBatch = s.planLimits.EffectiveLimits(workspaceID).MaxBatchSize
	} else if s.settings != nil {
		maxBatch = s.settings.MaxBatchSize()
	}
	if maxBatch > 0 && len(req.Recipients) > maxBatch {
		return nil, fmt.Errorf("batch size %d exceeds maximum allowed (%d)", len(req.Recipients), maxBatch)
	}

	from := req.From
	if from == "" {
		from = defaultFromAddress
	}

	// Validate sender address
	if err := validateEmailAddress(from); err != nil {
		return nil, fmt.Errorf("invalid sender: %w", err)
	}
	// Validate all recipient addresses
	for _, r := range req.Recipients {
		if err := validateEmailAddress(r.Email); err != nil {
			return nil, fmt.Errorf("invalid recipient: %w", err)
		}
	}

	tmpl, err := s.findTemplate(userID, workspaceID, req.TemplateID, req.Template)
	if err != nil {
		return nil, fmt.Errorf("template not found: %w", err)
	}

	resp := &BatchResponse{
		Total:   len(req.Recipients),
		Results: make([]BatchResult, 0, len(req.Recipients)),
	}

	for _, recipient := range req.Recipients {
		// Check suppression
		if s.suppressionRepo != nil {
			suppressed, err := s.suppressionRepo.IsSuppressed(repositories.ResourceScope{UserID: userID, WorkspaceID: workspaceID}, recipient.Email)
			if err == nil && suppressed {
				// Log the suppressed email
				var apiKeyPtr *uint
				if apiKeyID > 0 {
					apiKeyPtr = &apiKeyID
				}
				em := &models.Email{
					UserID:       userID,
					WorkspaceID:  workspaceID,
					APIKeyID:     apiKeyPtr,
					Sender:       from,
					Recipients:   []string{recipient.Email},
					Subject:      req.Template,
					Status:       models.EmailStatusSuppressed,
					ErrorMessage: "recipient is suppressed",
				}
				_ = s.emailRepo.Create(em)

				resp.Skipped++
				resp.Results = append(resp.Results, BatchResult{
					Email:  recipient.Email,
					ID:     em.UUID,
					Status: models.EmailStatusSuppressed,
					Error:  "recipient is suppressed",
				})
				continue
			}
		}

		// Use per-recipient language, fall back to batch-level language
		lang := recipient.Language
		if lang == "" {
			lang = req.Language
		}

		rendered, err := s.resolveAndRender(tmpl, lang, recipient.TemplateData)
		if err != nil {
			resp.Failed++
			resp.Results = append(resp.Results, BatchResult{
				Email:  recipient.Email,
				Status: models.EmailStatusFailed,
				Error:  fmt.Sprintf("template render failed: %v", err),
			})
			continue
		}

		sendResp, err := s.Send(ctx, userID, apiKeyID, workspaceID, userEmail, &SendRequest{
			From:    from,
			To:      []string{recipient.Email},
			Subject: rendered.Subject,
			HTML:    rendered.HTML,
			Text:    rendered.Text,
		})
		if err != nil {
			resp.Failed++
			resp.Results = append(resp.Results, BatchResult{
				Email:  recipient.Email,
				Status: models.EmailStatusFailed,
				Error:  err.Error(),
			})
			continue
		}

		resp.Sent++
		resp.Results = append(resp.Results, BatchResult{
			Email:  recipient.Email,
			ID:     sendResp.ID,
			Status: sendResp.Status,
		})
	}

	return resp, nil
}

// SendTestByTemplateID sends a test email using a template looked up by ID.
// It uses apiKeyID=0 since test sends come from the dashboard, not an API key.
func (s *Service) SendTestByTemplateID(ctx context.Context, userID uint, workspaceID *uint, userEmail string, templateID uint, req *SendTestRequest) (*SendResponse, error) {
	tmpl, err := s.templateRepo.FindByID(templateID)
	if err != nil {
		return nil, fmt.Errorf("template not found")
	}

	rendered, err := s.resolveAndRender(tmpl, req.Language, req.TemplateData)
	if err != nil {
		return nil, fmt.Errorf("failed to render template: %w", err)
	}

	from := req.From
	if from == "" {
		from = defaultFromAddress
	}

	return s.Send(ctx, userID, 0, workspaceID, userEmail, &SendRequest{
		From:    from,
		To:      req.To,
		Subject: rendered.Subject,
		HTML:    rendered.HTML,
		Text:    rendered.Text,
	})
}

type SendTestRequest struct {
	To           []string       `json:"to" required:"true" minItems:"1"`
	From         string         `json:"from"`
	Language     string         `json:"language"`
	TemplateData map[string]any `json:"template_data"`
}

// defaultRetryLimit is the fallback maximum number of manual retries when
// no SMTP server is configured or the server's MaxRetries is zero.
const defaultRetryLimit = 5

// RetryEmail re-enqueues a failed email for another delivery attempt.
// Only emails with status "failed" can be retried, and the retry count
// must not exceed the SMTP server's MaxRetries (or the default limit).
func (s *Service) RetryEmail(emailUUID string, userID uint, workspaceID *uint) (*SendResponse, error) {
	em, err := s.emailRepo.FindByUUID(emailUUID)
	if err != nil {
		return nil, fmt.Errorf("email not found")
	}
	if em.UserID != userID {
		return nil, fmt.Errorf("email not found")
	}
	// When called from a workspace-scoped context, verify the email belongs
	// to that workspace.
	if workspaceID != nil {
		if em.WorkspaceID == nil || *em.WorkspaceID != *workspaceID {
			return nil, fmt.Errorf("email not found")
		}
	}
	if em.Status != models.EmailStatusFailed {
		return nil, fmt.Errorf("only failed emails can be retried")
	}

	// Enforce retry limit from the SMTP server configuration.
	maxRetries := defaultRetryLimit
	if s.smtpRepo != nil {
		var smtpServer *models.SMTPServer
		if em.WorkspaceID != nil {
			smtpServer, _ = s.smtpRepo.FindFirstByWorkspaceID(*em.WorkspaceID)
		} else {
			smtpServer, _ = s.smtpRepo.FindFirstByUserID(em.UserID)
		}
		if smtpServer != nil && smtpServer.MaxRetries > 0 {
			maxRetries = smtpServer.MaxRetries
		}
	}
	if em.RetryCount >= maxRetries {
		return nil, fmt.Errorf("retry limit reached (%d/%d)", em.RetryCount, maxRetries)
	}

	em.RetryCount++

	if s.enqueuer != nil {
		em.Status = models.EmailStatusQueued
		em.ErrorMessage = ""
		_ = s.emailRepo.Update(em)
		if err := s.enqueuer.EnqueueEmailSend(em.ID, ""); err != nil {
			em.Status = models.EmailStatusFailed
			em.ErrorMessage = fmt.Sprintf("failed to enqueue: %v", err)
			_ = s.emailRepo.Update(em)
			return nil, fmt.Errorf("failed to enqueue email: %w", err)
		}
		return &SendResponse{ID: em.UUID, Status: em.Status}, nil
	}

	// Synchronous fallback
	return s.sendSync(em, userID, em.WorkspaceID, &SendRequest{
		From:                em.Sender,
		To:                  em.Recipients,
		Subject:             em.Subject,
		HTML:                em.HTMLBody,
		Text:                em.TextBody,
		ListUnsubscribeURL:  em.ListUnsubscribeURL,
		ListUnsubscribePost: em.ListUnsubscribePost,
	})
}

// RenderTemplate resolves a template by ID or name and renders it with the given data,
// returning the rendered subject, HTML, and text without sending.
func (s *Service) RenderTemplate(userID uint, workspaceID *uint, templateID *uint, templateName, language string, data map[string]any) (*RenderedTemplate, error) {
	tmpl, err := s.findTemplate(userID, workspaceID, templateID, templateName)
	if err != nil {
		return nil, fmt.Errorf("template not found: %w", err)
	}
	return s.resolveAndRender(tmpl, language, data)
}

// findTemplate looks up a template by ID or name. When templateID is provided
// it performs a fast primary-key lookup and verifies the caller owns the
// resource. Otherwise it falls back to a name-based search (workspace first,
// then personal).
func (s *Service) findTemplate(userID uint, workspaceID *uint, templateID *uint, name string) (*models.Template, error) {
	if templateID != nil && *templateID != 0 {
		tmpl, err := s.templateRepo.FindByID(*templateID)
		if err != nil {
			return nil, err
		}
		// Verify ownership: must belong to the same user/workspace.
		if workspaceID != nil {
			if tmpl.WorkspaceID == nil || *tmpl.WorkspaceID != *workspaceID {
				return nil, fmt.Errorf("template not found")
			}
		} else if tmpl.UserID != userID || tmpl.WorkspaceID != nil {
			return nil, fmt.Errorf("template not found")
		}
		return tmpl, nil
	}

	if name == "" {
		return nil, fmt.Errorf("template_id or template name is required")
	}

	if workspaceID != nil {
		tmpl, err := s.templateRepo.FindByWorkspaceName(*workspaceID, name)
		if err == nil {
			return tmpl, nil
		}
	}
	return s.templateRepo.FindByName(userID, name)
}

// resolveAndRender resolves the template content using versioned localizations
// and renders it with the given data.
func (s *Service) resolveAndRender(tmpl *models.Template, language string, data map[string]any) (*RenderedTemplate, error) {
	if tmpl.ActiveVersionID == nil {
		return nil, fmt.Errorf("template %q has no active version", tmpl.Name)
	}

	v, err := s.versionRepo.FindByID(*tmpl.ActiveVersionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load active version: %w", err)
	}

	l := s.resolveLocalization(v.ID, language, tmpl.DefaultLanguage)
	if l == nil {
		return nil, fmt.Errorf("no localization found for template %q", tmpl.Name)
	}

	var css string
	if v.StyleSheet != nil {
		css = v.StyleSheet.CSS
	}
	input := &RenderInput{
		SubjectTemplate: l.SubjectTemplate,
		HTMLTemplate:    l.HTMLTemplate,
		TextTemplate:    l.TextTemplate,
		CSS:             css,
	}
	return s.renderer.Render(input, data)
}

// resolveLocalization implements the language fallback strategy:
// requested language → base language (e.g. fr-CA → fr) → template default → "en"
func (s *Service) resolveLocalization(versionID uint, language, defaultLang string) *models.TemplateLocalization {
	if language == "" {
		language = defaultLang
	}
	if language == "" {
		language = "en"
	}

	// Try exact match
	l, err := s.localizationRepo.FindByVersionAndLanguage(versionID, language)
	if err == nil {
		return l
	}

	// Try base language (e.g. fr-CA → fr)
	if idx := strings.Index(language, "-"); idx > 0 {
		base := language[:idx]
		l, err = s.localizationRepo.FindByVersionAndLanguage(versionID, base)
		if err == nil {
			return l
		}
	}

	// Try template default language
	if language != defaultLang && defaultLang != "" {
		l, err = s.localizationRepo.FindByVersionAndLanguage(versionID, defaultLang)
		if err == nil {
			return l
		}
	}

	// Try "en" as final fallback
	if language != "en" && defaultLang != "en" {
		l, err = s.localizationRepo.FindByVersionAndLanguage(versionID, "en")
		if err == nil {
			return l
		}
	}

	return nil
}

// diffRecipients returns elements in original that are not in filtered.
func diffRecipients(original, filtered []string) []string {
	set := make(map[string]struct{}, len(filtered))
	for _, e := range filtered {
		set[e] = struct{}{}
	}
	var diff []string
	for _, e := range original {
		if _, ok := set[e]; !ok {
			diff = append(diff, e)
		}
	}
	return diff
}

// filterCustomHeaders removes headers that could conflict with system-set
// headers or be used for spoofing. It returns a sanitized copy.
func filterCustomHeaders(headers map[string]string) map[string]string {
	// Headers that are set by the system and must not be overridden.
	reserved := map[string]bool{
		"from":         true,
		"to":           true,
		"subject":      true,
		"mime-version": true,
		"content-type": true,
	}

	// Prefixes that must not appear in custom headers.
	blockedPrefixes := []string{
		"x-posta-",
		"dkim-",
		"arc-",
	}

	filtered := make(map[string]string, len(headers))
	for key, value := range headers {
		lower := strings.ToLower(key)
		if reserved[lower] {
			continue
		}
		blocked := false
		for _, prefix := range blockedPrefixes {
			if strings.HasPrefix(lower, prefix) {
				blocked = true
				break
			}
		}
		if blocked {
			continue
		}
		filtered[key] = value
	}
	return filtered
}

// checkDomainVerification enforces domain ownership verification when the user
// has RequireVerifiedDomain enabled. It extracts the sender domain and checks
// that it is registered and ownership-verified via TXT record.
func (s *Service) checkDomainVerification(userID uint, from string) error {
	if s.domainRepo == nil || s.userRepo == nil {
		return nil
	}

	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return fmt.Errorf("failed to load user: %w", err)
	}

	if !user.RequireVerifiedDomain {
		return nil
	}

	senderDomain := extractSenderDomain(from)
	if senderDomain == "" {
		return fmt.Errorf("domain_verification: could not extract domain from sender %q", from)
	}

	if !s.domainRepo.IsOwnershipVerified(userID, senderDomain) {
		return fmt.Errorf("domain_verification: domain %q is not verified. Add and verify the domain or disable strict domain mode", senderDomain)
	}

	return nil
}

// validateEmailAddress checks that a string is a valid RFC 5322 email address
func validateEmailAddress(addr string) error {
	parsed, err := mail.ParseAddress(addr)
	if err != nil {
		return fmt.Errorf("invalid email address %q: %w", addr, err)
	}
	parts := strings.SplitN(parsed.Address, "@", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("invalid email address %q: missing local part or domain", addr)
	}
	if !strings.Contains(parts[1], ".") {
		return fmt.Errorf("invalid email address %q: domain has no TLD", addr)
	}
	return nil
}

// validateAddresses validates the From and all To addresses.
func validateAddresses(from string, to []string) error {
	if err := validateEmailAddress(from); err != nil {
		return fmt.Errorf("invalid sender: %w", err)
	}
	for _, addr := range to {
		if err := validateEmailAddress(addr); err != nil {
			return fmt.Errorf("invalid recipient: %w", err)
		}
	}
	return nil
}

// extractSenderDomain returns the domain part of a sender address.
func extractSenderDomain(from string) string {
	bare := extractEmail(from)
	parts := strings.SplitN(bare, "@", 2)
	if len(parts) != 2 {
		return ""
	}
	return strings.ToLower(parts[1])
}

// extractEmail extracts the bare email address from a string that may be in
// RFC 5322 format like "Display Name <user@example.com>" or just "user@example.com".
func extractEmail(from string) string {
	addr, err := mail.ParseAddress(from)
	if err != nil {
		return from
	}
	return addr.Address
}
