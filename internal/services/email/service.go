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
	"context"
	"encoding/json"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/jkaninda/logger"
	"github.com/jkaninda/posta/internal/models"
	"github.com/jkaninda/posta/internal/services/ratelimit"
	"github.com/jkaninda/posta/internal/services/settings"
	"github.com/jkaninda/posta/internal/services/webhook"
	"github.com/jkaninda/posta/internal/storage/repositories"
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

// SetEnqueuer sets the email enqueuer for asynchronous delivery.
// When set, emails are enqueued to a background worker instead of being sent synchronously.
func (s *Service) SetEnqueuer(eq EmailEnqueuer) {
	s.enqueuer = eq
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
	Template     string              `json:"template" required:"true"`
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
	Template   string           `json:"template" required:"true"`
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
func (s *Service) ValidateSend(ctx context.Context, userID uint, userEmail string, req *SendRequest) (*DryRunResponse, error) {
	if s.settings != nil && s.settings.MaintenanceMode() {
		return nil, fmt.Errorf("maintenance: email sending is temporarily disabled")
	}

	if err := s.limiter.Check(ctx, userEmail); err != nil {
		return nil, fmt.Errorf("rate_limit: %w", err)
	}

	if len(req.Attachments) > 0 {
		maxSize := DefaultMaxAttachmentSize
		if s.settings != nil {
			maxSize = int64(s.settings.MaxAttachmentSizeMB()) * 1024 * 1024
		}
		if err := ValidateAttachments(req.Attachments, maxSize, DefaultMaxTotalSize); err != nil {
			return nil, fmt.Errorf("attachment validation: %w", err)
		}
	}

	if err := s.checkDomainVerification(userID, req.From); err != nil {
		return nil, err
	}

	activeRecipients := req.To
	suppressedCount := 0
	if s.suppressionRepo != nil {
		filtered, err := s.suppressionRepo.FilterSuppressed(userID, req.To)
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
func (s *Service) ValidateSendWithTemplate(ctx context.Context, userID uint, userEmail string, req *SendTemplateRequest) (*DryRunResponse, error) {
	tmpl, err := s.templateRepo.FindByName(userID, req.Template)
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

	resp, err := s.ValidateSend(ctx, userID, userEmail, &SendRequest{
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
func (s *Service) ValidateSendBatch(ctx context.Context, userID uint, userEmail string, req *BatchRequest) (*DryRunResponse, error) {
	if s.settings != nil {
		maxBatch := s.settings.MaxBatchSize()
		if maxBatch > 0 && len(req.Recipients) > maxBatch {
			return nil, fmt.Errorf("batch size %d exceeds maximum allowed (%d)", len(req.Recipients), maxBatch)
		}
	}

	_, err := s.templateRepo.FindByName(userID, req.Template)
	if err != nil {
		return nil, fmt.Errorf("template not found: %s", req.Template)
	}

	from := req.From
	if from == "" {
		from = defaultFromAddress
	}

	emails := make([]string, len(req.Recipients))
	for i, r := range req.Recipients {
		emails[i] = r.Email
	}

	activeRecipients := emails
	suppressedCount := 0
	if s.suppressionRepo != nil {
		filtered, err := s.suppressionRepo.FilterSuppressed(userID, emails)
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

func (s *Service) Send(ctx context.Context, userID, apiKeyID uint, userEmail string, req *SendRequest) (*SendResponse, error) {
	// Enforce maintenance mode
	if s.settings != nil && s.settings.MaintenanceMode() {
		return nil, fmt.Errorf("maintenance: email sending is temporarily disabled")
	}

	if err := s.limiter.Allow(ctx, userEmail); err != nil {
		return nil, fmt.Errorf("rate_limit: %w", err)
	}

	// Validate attachments if present
	if len(req.Attachments) > 0 {
		maxSize := DefaultMaxAttachmentSize
		if s.settings != nil {
			maxSize = int64(s.settings.MaxAttachmentSizeMB()) * 1024 * 1024
		}
		if err := ValidateAttachments(req.Attachments, maxSize, DefaultMaxTotalSize); err != nil {
			return nil, fmt.Errorf("attachment validation: %w", err)
		}
	}

	// Enforce verified domain when user has strict mode enabled
	if err := s.checkDomainVerification(userID, req.From); err != nil {
		return nil, err
	}

	// Filter out suppressed recipients
	if s.suppressionRepo != nil {
		filtered, err := s.suppressionRepo.FilterSuppressed(userID, req.To)
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

	// Serialize attachments metadata for storage
	var attachmentsJSON string
	if len(req.Attachments) > 0 {
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
	return s.sendSync(em, userID, req)
}

// sendSync performs synchronous SMTP delivery (used when no enqueuer is configured).
func (s *Service) sendSync(em *models.Email, userID uint, req *SendRequest) (*SendResponse, error) {
	smtpServer, err := s.smtpRepo.FindFirstByUserID(userID)
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
			go s.contactRepo.RecordFailed(userID, req.To)
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
		go s.contactRepo.RecordSent(userID, req.To)
	}

	return &SendResponse{ID: em.UUID, Status: em.Status}, nil
}

func (s *Service) SendWithTemplate(ctx context.Context, userID, apiKeyID uint, userEmail string, req *SendTemplateRequest) (*SendResponse, error) {
	tmpl, err := s.templateRepo.FindByName(userID, req.Template)
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

	return s.Send(ctx, userID, apiKeyID, userEmail, &SendRequest{
		From:        from,
		To:          req.To,
		Subject:     rendered.Subject,
		HTML:        rendered.HTML,
		Text:        rendered.Text,
		Attachments: req.Attachments,
	})
}

func (s *Service) SendBatch(ctx context.Context, userID, apiKeyID uint, userEmail string, req *BatchRequest) (*BatchResponse, error) {
	// Enforce max batch size from platform settings
	if s.settings != nil {
		maxBatch := s.settings.MaxBatchSize()
		if maxBatch > 0 && len(req.Recipients) > maxBatch {
			return nil, fmt.Errorf("batch size %d exceeds maximum allowed (%d)", len(req.Recipients), maxBatch)
		}
	}

	tmpl, err := s.templateRepo.FindByName(userID, req.Template)
	if err != nil {
		return nil, fmt.Errorf("template not found: %s", req.Template)
	}

	from := req.From
	if from == "" {
		from = defaultFromAddress
	}

	resp := &BatchResponse{
		Total:   len(req.Recipients),
		Results: make([]BatchResult, 0, len(req.Recipients)),
	}

	for _, recipient := range req.Recipients {
		// Check suppression
		if s.suppressionRepo != nil {
			suppressed, err := s.suppressionRepo.IsSuppressed(userID, recipient.Email)
			if err == nil && suppressed {
				// Log the suppressed email
				var apiKeyPtr *uint
				if apiKeyID > 0 {
					apiKeyPtr = &apiKeyID
				}
				em := &models.Email{
					UserID:       userID,
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

		sendResp, err := s.Send(ctx, userID, apiKeyID, userEmail, &SendRequest{
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
func (s *Service) SendTestByTemplateID(ctx context.Context, userID uint, userEmail string, templateID uint, req *SendTestRequest) (*SendResponse, error) {
	tmpl, err := s.templateRepo.FindByID(templateID)
	if err != nil || tmpl.UserID != userID {
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

	return s.Send(ctx, userID, 0, userEmail, &SendRequest{
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

// RetryEmail re-enqueues a failed email for another delivery attempt.
// Only emails with status "failed" can be retried.
func (s *Service) RetryEmail(emailUUID string, userID uint) (*SendResponse, error) {
	em, err := s.emailRepo.FindByUUID(emailUUID)
	if err != nil {
		return nil, fmt.Errorf("email not found")
	}
	if em.UserID != userID {
		return nil, fmt.Errorf("email not found")
	}
	if em.Status != models.EmailStatusFailed {
		return nil, fmt.Errorf("only failed emails can be retried")
	}

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
	return s.sendSync(em, userID, &SendRequest{
		From:                em.Sender,
		To:                  em.Recipients,
		Subject:             em.Subject,
		HTML:                em.HTMLBody,
		Text:                em.TextBody,
		ListUnsubscribeURL:  em.ListUnsubscribeURL,
		ListUnsubscribePost: em.ListUnsubscribePost,
	})
}

// RenderTemplate resolves a template by name and renders it with the given data,
// returning the rendered subject, HTML, and text without sending.
func (s *Service) RenderTemplate(userID uint, templateName, language string, data map[string]any) (*RenderedTemplate, error) {
	tmpl, err := s.templateRepo.FindByName(userID, templateName)
	if err != nil {
		return nil, fmt.Errorf("template not found: %s", templateName)
	}
	return s.resolveAndRender(tmpl, language, data)
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
