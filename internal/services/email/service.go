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

package email

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/ratelimit"
	"github.com/goposta/posta/internal/services/settings"
	"github.com/goposta/posta/internal/services/webhook"
	"github.com/goposta/posta/internal/storage/blob"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/logger"
)

const (
	DefaultMaxAttachmentSize int64 = 10 * 1024 * 1024
	DefaultMaxTotalSize      int64 = 25 * 1024 * 1024

	defaultFromAddress = "noreply@localhost"
)

// EmailEnqueuer, when set on the Service, makes delivery asynchronous via a worker.
type EmailEnqueuer interface {
	EnqueueEmailSend(emailID uint, queue string) error
	EnqueueEmailSendAt(emailID uint, queue string, sendAt time.Time) error
}

type PlanLimitsProvider interface {
	EffectiveLimits(workspaceID *uint) *PlanLimits
}

type PlanLimits struct {
	HourlyRateLimit     int
	DailyRateLimit      int
	MaxAttachmentSizeMB int
	MaxBatchSize        int
}

// LinkGenerator stays narrow so the email service does not depend on the tracking package.
type LinkGenerator interface {
	TxUnsubscribeURL(emailID uint) string
	WebViewURL(emailUUID string) string
}

type Service struct {
	emailRepo        *repositories.EmailRepository
	smtpRepo         *repositories.SMTPRepository
	templateRepo     *repositories.TemplateRepository
	suppressionRepo  *repositories.SuppressionRepository
	unsubRepo        *repositories.UnsubscribeListRepository
	versionRepo      *repositories.TemplateVersionRepository
	localizationRepo *repositories.TemplateLocalizationRepository
	contactRepo      *repositories.ContactRepository
	domainRepo       *repositories.DomainRepository
	userRepo         *repositories.UserRepository
	wsSettingRepo    *repositories.WorkspaceSettingRepository
	sender           *SMTPSender
	renderer         *TemplateRenderer
	limiter          *ratelimit.RedisLimiter
	dispatcher       *webhook.Dispatcher
	enqueuer         EmailEnqueuer
	settings         *settings.Provider
	blobStore        blob.Store
	planLimits       PlanLimitsProvider
	linkGen          LinkGenerator
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

func (s *Service) SetVersionRepos(vr *repositories.TemplateVersionRepository, lr *repositories.TemplateLocalizationRepository) {
	s.versionRepo = vr
	s.localizationRepo = lr
}

func (s *Service) SetContactRepo(cr *repositories.ContactRepository) {
	s.contactRepo = cr
}

func (s *Service) SetUnsubscribeListRepo(r *repositories.UnsubscribeListRepository) {
	s.unsubRepo = r
}

// SetDomainVerification reads strict mode from workspace settings, falling back to the user's in personal mode.
func (s *Service) SetDomainVerification(dr *repositories.DomainRepository, ur *repositories.UserRepository, wsr *repositories.WorkspaceSettingRepository) {
	s.domainRepo = dr
	s.userRepo = ur
	s.wsSettingRepo = wsr
}

func (s *Service) SetSettings(sp *settings.Provider) {
	s.settings = sp
}

func (s *Service) SetPlanLimits(pl PlanLimitsProvider) {
	s.planLimits = pl
}

func (s *Service) SetEnqueuer(eq EmailEnqueuer) {
	s.enqueuer = eq
}

// SetLinkGenerator wires the links behind the reserved {{ posta_* }} template variables.
func (s *Service) SetLinkGenerator(gen LinkGenerator) {
	s.linkGen = gen
}

// SetBlobStore sends attachment content to the store and keeps only a storage key in the database.
func (s *Service) SetBlobStore(bs blob.Store) {
	s.blobStore = bs
}

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

func (s *Service) DownloadAttachments(ctx context.Context, attachments []models.Attachment) ([]models.Attachment, error) {
	return fetchAttachmentContent(ctx, s.blobStore, attachments)
}

func fetchAttachmentContent(ctx context.Context, store blob.Store, attachments []models.Attachment) ([]models.Attachment, error) {
	result := make([]models.Attachment, len(attachments))
	for i, att := range attachments {
		result[i] = att
		if att.StorageKey == "" {
			continue
		}
		rc, err := store.Get(ctx, att.StorageKey)
		if err != nil {
			return nil, fmt.Errorf("failed to download attachment %q: %w", att.Filename, err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read attachment %q: %w", att.Filename, err)
		}
		result[i].Content = base64.StdEncoding.EncodeToString(data)
	}
	return result, nil
}

// Without a blob store the base64 content stays on the record, because the record is all the send worker gets.
func (s *Service) persistAttachments(ctx context.Context, keyPrefix string, atts []models.Attachment) (string, error) {
	if len(atts) == 0 {
		return "", nil
	}
	stored := atts
	if s.blobStore != nil {
		uploaded, err := s.uploadAttachments(ctx, keyPrefix, atts)
		if err != nil {
			return "", fmt.Errorf("attachment upload: %w", err)
		}
		stored = uploaded
	}
	b, _ := json.Marshal(stored)
	return string(b), nil
}

// LoadAttachments must recover everything from the record: whatever it cannot goes out as a zero-byte file.
func LoadAttachments(ctx context.Context, em *models.Email, store blob.Store) ([]models.Attachment, error) {
	if em.AttachmentsJSON == "" {
		return nil, nil
	}
	var atts []models.Attachment
	if err := json.Unmarshal([]byte(em.AttachmentsJSON), &atts); err != nil {
		return nil, fmt.Errorf("decode attachments: %w", err)
	}
	if store == nil {
		return atts, nil
	}
	return fetchAttachmentContent(ctx, store, atts)
}

func (s *Service) DeleteAttachments(ctx context.Context, attachments []models.Attachment) {
	for _, att := range attachments {
		if att.StorageKey != "" {
			_ = s.blobStore.Delete(ctx, att.StorageKey)
		}
	}
}

func (s *Service) OnSent(fn func()) { s.onSent = fn }

func (s *Service) OnFailed(fn func()) { s.onFailed = fn }

func (s *Service) OnQueued(fn func()) { s.onQueued = fn }

var (
	ErrUnsubscribeListNotFound = errors.New("unsubscribe list not found in scope")
	ErrUnsubscribeInvalid      = errors.New("invalid unsubscribe configuration")
)

type Unsubscribe struct {
	ListID *uint `json:"list_id,omitempty" doc:"Reference an existing UnsubscribeList by id (Posta-managed). Posta mints the signed one-click URL and a click suppresses the recipient on this list only. Mutually exclusive with url."`

	// URL is also the RFC 8058 POST target when OneClick is set.
	URL    string `json:"url,omitempty" format:"url" doc:"Caller-managed unsubscribe endpoint. Posta only emits the List-Unsubscribe header; you own the endpoint. Also the RFC 8058 POST target when one_click is true (which requires https). Mutually exclusive with list_id."`
	Mailto string `json:"mailto,omitempty" doc:"Optional mailto: URI emitted alongside the URL in the List-Unsubscribe header (RFC 2369). A bare address is accepted; Posta prepends 'mailto:' if missing."`
	// OneClick requires an https URL on the caller-managed path.
	OneClick bool `json:"one_click,omitempty" doc:"Emit 'List-Unsubscribe-Post: List-Unsubscribe=One-Click' (RFC 8058). Applies to the https URL target only; the caller-managed path requires an https url. Implied true on the Posta-managed (list_id) path."`
}

const errRecipientSuppressed = "recipient is suppressed"

type SendRequest struct {
	From        string              `json:"from" required:"true" doc:"Sender address. Accepts a plain address (hello@example.com) or RFC 5322 display-name format (Acme <hello@example.com>)."`
	To          []string            `json:"to" required:"true" minItems:"1" doc:"Recipient addresses. Each accepts a plain address or RFC 5322 display-name format (e.g. Jonas <jonas@example.com>)."`
	Subject     string              `json:"subject" required:"true"`
	HTML        string              `json:"html"`
	Text        string              `json:"text"`
	Attachments []models.Attachment `json:"attachments,omitempty" doc:"Optional file attachments. Each must include filename, content_type, and base64-encoded content. Subject to per-attachment and total size limits set by your plan."`
	Headers     map[string]string   `json:"headers,omitempty"`
	// SystemHeaders are never accepted from API callers and apply whether or not custom headers are allowed.
	SystemHeaders map[string]string `json:"-"`
	Unsubscribe   *Unsubscribe      `json:"unsubscribe,omitempty"`
	// Deprecated: use Unsubscribe; mapped onto Unsubscribe{URL, OneClick} in resolveUnsubscribe.
	ListUnsubscribeURL  string     `json:"list_unsubscribe_url,omitempty" deprecated:"true"`
	ListUnsubscribePost bool       `json:"list_unsubscribe_post,omitempty" deprecated:"true"`
	SendAt              *time.Time `json:"send_at,omitempty"`
	TemplateName        string     `json:"-"`
}

type resolvedUnsub struct {
	listID   *uint
	url      string // caller-managed https URL
	mailto   string
	oneClick bool
}

func (s *Service) resolveUnsubscribe(scope repositories.ResourceScope, req *SendRequest) (*resolvedUnsub, error) {
	u := req.Unsubscribe
	// Map legacy fields onto the object when the new one is absent.
	if u == nil {
		if req.ListUnsubscribeURL == "" && !req.ListUnsubscribePost {
			return nil, nil
		}
		logger.Warn("list_unsubscribe_url/list_unsubscribe_post are deprecated; use the unsubscribe object")
		u = &Unsubscribe{URL: req.ListUnsubscribeURL, OneClick: req.ListUnsubscribePost}
	}

	if u.ListID != nil && u.URL != "" {
		return nil, fmt.Errorf("%w: list_id and url are mutually exclusive", ErrUnsubscribeInvalid)
	}

	mailto := strings.TrimSpace(u.Mailto)
	if mailto != "" && !strings.HasPrefix(strings.ToLower(mailto), "mailto:") {
		mailto = "mailto:" + mailto
	}

	// The one-click URL for a Posta-managed list is minted later, since it needs the email id.
	if u.ListID != nil {
		if s.unsubRepo == nil {
			return nil, fmt.Errorf("%w: list-managed unsubscribe is not configured", ErrUnsubscribeInvalid)
		}
		if _, err := s.unsubRepo.FindByIDForScope(*u.ListID, scope); err != nil {
			return nil, fmt.Errorf("%w: list_id %d", ErrUnsubscribeListNotFound, *u.ListID)
		}
		return &resolvedUnsub{listID: u.ListID, mailto: mailto, oneClick: true}, nil
	}

	if u.URL == "" {
		if mailto == "" {
			return nil, nil
		}
		return &resolvedUnsub{mailto: mailto}, nil
	}
	if u.OneClick && !strings.HasPrefix(strings.ToLower(u.URL), "https://") {
		return nil, fmt.Errorf("%w: one_click requires an https url", ErrUnsubscribeInvalid)
	}
	return &resolvedUnsub{url: u.URL, mailto: mailto, oneClick: u.OneClick}, nil
}

type SendTemplateRequest struct {
	Template     TemplateRef         `json:"template" doc:"Template name or ID. A name is matched first; a value that is all digits and matches no name is looked up as an ID."`
	TemplateID   *uint               `json:"template_id,omitempty" deprecated:"true" doc:"Deprecated: pass the ID in template instead. Used only when template is empty."`
	Language     string              `json:"language" doc:"Language code for template localization (BCP 47 / ISO 639-1, e.g. en, fr, fr-CA). Falls back to base language, the template default, then en."`
	From         string              `json:"from" doc:"Sender address. Accepts a plain address (hello@example.com) or RFC 5322 display-name format (Acme <hello@example.com>)."`
	To           []string            `json:"to" required:"true" minItems:"1" doc:"Recipient addresses. Each accepts a plain address or RFC 5322 display-name format (e.g. Jonas <jonas@example.com>)."`
	TemplateData map[string]any      `json:"template_data" doc:"Variables substituted into the template's {{ var_name }} placeholders. Reserved posta_* variables (e.g. posta_unsubscribe_url, posta_web_view_url) are injected automatically."`
	Attachments  []models.Attachment `json:"attachments,omitempty" doc:"Optional file attachments. Each must include filename, content_type, and base64-encoded content. Subject to per-attachment and total size limits set by your plan."`
	Unsubscribe  *Unsubscribe        `json:"unsubscribe,omitempty"`
}

type SendResponse struct {
	ID     string             `json:"id"`
	Status models.EmailStatus `json:"status"`
}

type BatchRequest struct {
	Template    TemplateRef      `json:"template" doc:"Template name or ID. A name is matched first; a value that is all digits and matches no name is looked up as an ID."`
	TemplateID  *uint            `json:"template_id,omitempty" deprecated:"true" doc:"Deprecated: pass the ID in template instead. Used only when template is empty."`
	Language    string           `json:"language" doc:"Default language for the batch (BCP 47 / ISO 639-1, e.g. en, fr, fr-CA). Each recipient can override via recipients[].language."`
	From        string           `json:"from" doc:"Sender address. Accepts a plain address (hello@example.com) or RFC 5322 display-name format (Acme <hello@example.com>)."`
	Recipients  []BatchRecipient `json:"recipients" required:"true" minItems:"1"`
	Unsubscribe *Unsubscribe     `json:"unsubscribe,omitempty"`
}

type BatchRecipient struct {
	Email        string         `json:"email" required:"true" format:"email" doc:"Recipient address. Accepts a plain address or RFC 5322 display-name format (e.g. Jonas <jonas@example.com>)."`
	Language     string         `json:"language" doc:"Per-recipient language override (BCP 47 / ISO 639-1). Falls back to the batch-level language, then the template default."`
	TemplateData map[string]any `json:"template_data" doc:"Variables substituted into the template's {{ var_name }} placeholders. Reserved posta_* variables (e.g. posta_unsubscribe_url, posta_web_view_url) are injected automatically."`
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

func (s *Service) ValidateSend(ctx context.Context, userID uint, workspaceID *uint, userEmail string, req *SendRequest) (*DryRunResponse, error) {
	if err := validateAddresses(req.From, req.To); err != nil {
		return nil, err
	}
	req.From = s.applyDefaultSenderName(workspaceID, req.From)

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

	if err := s.checkDomainVerification(userID, workspaceID, req.From); err != nil {
		return nil, err
	}

	scope := repositories.ResourceScope{UserID: userID, WorkspaceID: workspaceID}
	unsub, err := s.resolveUnsubscribe(scope, req)
	if err != nil {
		return nil, err
	}
	var unsubListID *uint
	if unsub != nil {
		unsubListID = unsub.listID
	}
	activeRecipients := req.To
	suppressedCount := 0
	if s.suppressionRepo != nil {
		filtered, err := s.suppressionRepo.FilterSuppressedForList(scope, req.To, unsubListID)
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
		Unsubscribe: req.Unsubscribe,
	})
	if err != nil {
		return nil, err
	}
	resp.RenderedSubject = rendered.Subject
	return resp, nil
}

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
	unsub, err := s.resolveUnsubscribe(scope, &SendRequest{Unsubscribe: req.Unsubscribe})
	if err != nil {
		return nil, err
	}
	var unsubListID *uint
	if unsub != nil {
		unsubListID = unsub.listID
	}
	activeRecipients := emails
	suppressedCount := 0
	if s.suppressionRepo != nil {
		filtered, err := s.suppressionRepo.FilterSuppressedForList(scope, emails, unsubListID)
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
		Subject:         req.Template.String(),
		Recipients:      len(activeRecipients),
		SuppressedCount: suppressedCount,
	}, nil
}

func (s *Service) Send(ctx context.Context, userID, apiKeyID uint, workspaceID *uint, userEmail string, req *SendRequest) (*SendResponse, error) {
	if err := validateAddresses(req.From, req.To); err != nil {
		return nil, err
	}

	req.From = s.applyDefaultSenderName(workspaceID, req.From)

	if s.settings != nil && s.settings.MaintenanceMode() {
		return nil, fmt.Errorf("maintenance: email sending is temporarily disabled")
	}

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

	// Strict mode is scoped to the active workspace, or the user in legacy personal mode.
	if err := s.checkDomainVerification(userID, workspaceID, req.From); err != nil {
		return nil, err
	}

	// A nil unsubscribe config means no headers and no suppression, the transactional default.
	scope := repositories.ResourceScope{UserID: userID, WorkspaceID: workspaceID}
	unsub, err := s.resolveUnsubscribe(scope, req)
	if err != nil {
		return nil, err
	}
	var unsubListID *uint
	if unsub != nil {
		unsubListID = unsub.listID
	}

	// Filter out suppressed recipients (list-scoped when a list is named)
	if s.suppressionRepo != nil {
		filtered, err := s.suppressionRepo.FilterSuppressedForList(scope, req.To, unsubListID)
		if err != nil {
			return nil, fmt.Errorf("failed to check suppression list: %w", err)
		}
		if len(filtered) == 0 {
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
				TemplateName: req.TemplateName,
				HTMLBody:     req.HTML,
				TextBody:     req.Text,
				Status:       models.EmailStatusSuppressed,
				ErrorMessage: "all recipients are suppressed",
				Provider:     ClassifyRecipients(req.To),
			}
			_ = s.emailRepo.Create(em)
			return &SendResponse{ID: em.UUID, Status: em.Status}, nil
		}
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
					TemplateName: req.TemplateName,
					Status:       models.EmailStatusSuppressed,
					ErrorMessage: errRecipientSuppressed,
					Provider:     ClassifyProvider(addr),
				}
				_ = s.emailRepo.Create(em)
			}
		}
		req.To = filtered
	}

	// The blob key prefix stands in for the email UUID, which is not assigned until the record is created.
	attachmentsJSON, err := s.persistAttachments(ctx, fmt.Sprintf("%d", time.Now().UnixNano()), req.Attachments)
	if err != nil {
		return nil, err
	}

	if req.Text == "" && req.HTML != "" {
		req.Text = HTMLToText(req.HTML)
	}

	// Resolve headers once so the async worker, reading them from the row, and the sync path agree.
	customEnabled := s.settings != nil && s.settings.CustomHeadersEnabled()
	req.Headers = outboundHeaders(req, customEnabled)
	var headersJSON string
	if len(req.Headers) > 0 {
		b, _ := json.Marshal(req.Headers)
		headersJSON = string(b)
	}

	var apiKeyPtr *uint
	if apiKeyID > 0 {
		apiKeyPtr = &apiKeyID
	}

	// The Posta-managed list URL is minted after Create since it needs em.ID.
	var luURL, luMailto string
	var luPost bool
	if unsub != nil {
		luMailto = unsub.mailto
		if unsub.url != "" {
			luURL = unsub.url
			luPost = unsub.oneClick
		}
	}

	em := &models.Email{
		UUID:                  uuid.NewString(),
		UserID:                userID,
		WorkspaceID:           workspaceID,
		APIKeyID:              apiKeyPtr,
		Sender:                req.From,
		Recipients:            req.To,
		Subject:               req.Subject,
		TemplateName:          req.TemplateName,
		HTMLBody:              req.HTML,
		TextBody:              req.Text,
		AttachmentsJSON:       attachmentsJSON,
		HeadersJSON:           headersJSON,
		ListUnsubscribeURL:    luURL,
		ListUnsubscribeMailto: luMailto,
		ListUnsubscribePost:   luPost,
		UnsubscribeListID:     unsubListID,
		Status:                models.EmailStatusPending,
		Provider:              ClassifyRecipients(req.To),
	}

	if err := s.emailRepo.Create(em); err != nil {
		return nil, fmt.Errorf("failed to store email: %w", err)
	}

	needsLinks := HasSystemSentinels(em.HTMLBody) || HasSystemSentinels(em.TextBody) || HasSystemSentinels(em.Subject)
	if s.linkGen != nil && (needsLinks || em.UnsubscribeListID != nil) {
		webViewURL := s.linkGen.WebViewURL(em.UUID)
		unsubscribeURL := s.linkGen.TxUnsubscribeURL(em.ID)
		if needsLinks {
			em.HTMLBody = SubstituteSystemLinks(em.HTMLBody, webViewURL, unsubscribeURL)
			em.TextBody = SubstituteSystemLinks(em.TextBody, webViewURL, unsubscribeURL)
			em.Subject = SubstituteSystemLinks(em.Subject, webViewURL, unsubscribeURL)

			req.HTML = em.HTMLBody
			req.Text = em.TextBody
			req.Subject = em.Subject
		}
		if em.UnsubscribeListID != nil {
			em.ListUnsubscribeURL = unsubscribeURL
			em.ListUnsubscribePost = true
		}
		_ = s.emailRepo.Update(em)
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

	return s.sendSync(em, userID, workspaceID, req)
}

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
		s.dispatcher.Dispatch(userID, workspaceID, "email.failed", em.UUID, req.From)
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
			s.dispatcher.Dispatch(userID, workspaceID, "email.failed", em.UUID, req.From)
			if s.onFailed != nil {
				s.onFailed()
			}
			return nil, fmt.Errorf("sender %q is not in the allowed emails list", req.From)
		}
	}

	em.SMTPHostname = smtpServer.Host
	_ = s.emailRepo.Update(em)

	if err := s.sender.Send(smtpServer, req.From, req.To, req.Subject, req.HTML, req.Text, req.Attachments, req.Headers, em.ListUnsubscribeURL, em.ListUnsubscribeMailto, em.ListUnsubscribePost); err != nil {
		em.Status = models.EmailStatusFailed
		em.ErrorMessage = err.Error()
		_ = s.emailRepo.Update(em)
		s.dispatcher.Dispatch(userID, workspaceID, "email.failed", em.UUID, req.From)
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
	s.dispatcher.Dispatch(userID, workspaceID, "email.sent", em.UUID, req.From)
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
		From:         from,
		To:           req.To,
		Subject:      rendered.Subject,
		HTML:         rendered.HTML,
		Text:         rendered.Text,
		Attachments:  req.Attachments,
		TemplateName: tmpl.Name,
		Unsubscribe:  req.Unsubscribe,
	})
}

func (s *Service) SendBatch(ctx context.Context, userID, apiKeyID uint, workspaceID *uint, userEmail string, req *BatchRequest) (*BatchResponse, error) {
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

	if err := validateEmailAddress(from); err != nil {
		return nil, fmt.Errorf("invalid sender: %w", err)
	}
	for _, r := range req.Recipients {
		if err := validateEmailAddress(r.Email); err != nil {
			return nil, fmt.Errorf("invalid recipient: %w", err)
		}
	}

	tmpl, err := s.findTemplate(userID, workspaceID, req.TemplateID, req.Template)
	if err != nil {
		return nil, fmt.Errorf("template not found: %w", err)
	}

	// Resolve unsubscribe once so a bad list_id fails the whole batch instead of every row.
	scope := repositories.ResourceScope{UserID: userID, WorkspaceID: workspaceID}
	unsub, err := s.resolveUnsubscribe(scope, &SendRequest{Unsubscribe: req.Unsubscribe})
	if err != nil {
		return nil, err
	}
	var unsubListID *uint
	if unsub != nil {
		unsubListID = unsub.listID
	}

	resp := &BatchResponse{
		Total:   len(req.Recipients),
		Results: make([]BatchResult, 0, len(req.Recipients)),
	}

	for _, recipient := range req.Recipients {
		// Check suppression (list-scoped when a list is named)
		if s.suppressionRepo != nil {
			suppressed, err := s.suppressionRepo.IsSuppressedForList(scope, recipient.Email, unsubListID)
			if err == nil && suppressed {
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
					Subject:      req.Template.String(),
					TemplateName: tmpl.Name,
					Status:       models.EmailStatusSuppressed,
					ErrorMessage: errRecipientSuppressed,
					Provider:     ClassifyProvider(recipient.Email),
				}
				_ = s.emailRepo.Create(em)

				resp.Skipped++
				resp.Results = append(resp.Results, BatchResult{
					Email:  recipient.Email,
					ID:     em.UUID,
					Status: models.EmailStatusSuppressed,
					Error:  errRecipientSuppressed,
				})
				continue
			}
		}

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
			From:         from,
			To:           []string{recipient.Email},
			Subject:      rendered.Subject,
			HTML:         rendered.HTML,
			Text:         rendered.Text,
			TemplateName: tmpl.Name,
			Unsubscribe:  req.Unsubscribe,
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

// SendTestByTemplateID uses apiKeyID=0 since test sends come from the dashboard.
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
		From:         from,
		To:           req.To,
		Subject:      rendered.Subject,
		HTML:         rendered.HTML,
		Text:         rendered.Text,
		TemplateName: tmpl.Name,
	})
}

type SendTestRequest struct {
	To           []string       `json:"to" required:"true" minItems:"1" doc:"Recipient addresses. Each accepts a plain address or RFC 5322 display-name format (e.g. Jonas <jonas@example.com>)."`
	From         string         `json:"from" doc:"Sender address. Accepts a plain address (hello@example.com) or RFC 5322 display-name format (Acme <hello@example.com>)."`
	Language     string         `json:"language" doc:"Language code for template localization (BCP 47 / ISO 639-1, e.g. en, fr, fr-CA). Falls back to base language, the template default, then en."`
	TemplateData map[string]any `json:"template_data" doc:"Variables substituted into the template's {{ var_name }} placeholders. Reserved posta_* variables (e.g. posta_unsubscribe_url, posta_web_view_url) are injected automatically."`
}

// defaultRetryLimit applies when no SMTP server is configured or its MaxRetries is zero.
const defaultRetryLimit = 5

// RetryEmail only retries "failed" emails, up to the SMTP server's MaxRetries or the default limit.
func (s *Service) RetryEmail(emailUUID string, userID uint, workspaceID *uint) (*SendResponse, error) {
	em, err := s.emailRepo.FindByUUID(emailUUID)
	if err != nil {
		return nil, fmt.Errorf("email not found")
	}
	if em.UserID != userID {
		return nil, fmt.Errorf("email not found")
	}
	if workspaceID != nil {
		if em.WorkspaceID == nil || *em.WorkspaceID != *workspaceID {
			return nil, fmt.Errorf("email not found")
		}
	}
	if em.Status != models.EmailStatusFailed {
		return nil, fmt.Errorf("only failed emails can be retried")
	}

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

func (s *Service) RenderTemplate(userID uint, workspaceID *uint, templateID *uint, ref TemplateRef, language string, data map[string]any) (*RenderedTemplate, error) {
	tmpl, err := s.findTemplate(userID, workspaceID, templateID, ref)
	if err != nil {
		return nil, fmt.Errorf("template not found: %w", err)
	}
	rendered, err := s.resolveAndRender(tmpl, language, data)
	if err != nil {
		return nil, err
	}
	// Previews have no real message, so show reserved variables by name, not sentinels or throwaway links.
	rendered.HTML = SubstituteSystemLinks(rendered.HTML, VarWebView, VarUnsubscribe)
	rendered.Text = SubstituteSystemLinks(rendered.Text, VarWebView, VarUnsubscribe)
	rendered.Subject = SubstituteSystemLinks(rendered.Subject, VarWebView, VarUnsubscribe)
	return rendered, nil
}

// findTemplate matches names before all-digit IDs, so a template named "2024" wins over ID 2024.
func (s *Service) findTemplate(userID uint, workspaceID *uint, templateID *uint, ref TemplateRef) (*models.Template, error) {
	if ref == "" {
		if templateID != nil && *templateID != 0 {
			return s.findTemplateByID(userID, workspaceID, *templateID)
		}
		return nil, fmt.Errorf("template is required")
	}

	if workspaceID != nil {
		if tmpl, err := s.templateRepo.FindByWorkspaceName(*workspaceID, ref.String()); err == nil {
			return tmpl, nil
		}
	}
	if id, ok := ref.ID(); ok {
		if tmpl, err := s.findTemplateByID(userID, workspaceID, id); err == nil {
			return tmpl, nil
		}
	}
	if workspaceID != nil {
		// Before 0.18 a template's name was its label, and callers sent that.
		if tmpl, err := s.templateRepo.FindByWorkspaceDisplayName(*workspaceID, ref.String()); err == nil {
			logger.Warn("template sent by its display name, which is deprecated: send its name instead",
				"display_name", ref.String(), "name", tmpl.Name, "workspace_id", *workspaceID)
			return tmpl, nil
		}
	}
	if workspaceID == nil {
		return nil, fmt.Errorf("template %q not found: no active workspace", ref)
	}
	return nil, fmt.Errorf("template %q not found", ref)
}

// findTemplateByID requires the template to be in the caller's workspace, or their own personal one.
func (s *Service) findTemplateByID(userID uint, workspaceID *uint, id uint) (*models.Template, error) {
	tmpl, err := s.templateRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if workspaceID != nil {
		if tmpl.WorkspaceID == nil || *tmpl.WorkspaceID != *workspaceID {
			return nil, fmt.Errorf("template not found")
		}
	} else if tmpl.UserID != userID || tmpl.WorkspaceID != nil {
		return nil, fmt.Errorf("template not found")
	}
	return tmpl, nil
}

func (s *Service) resolveAndRender(tmpl *models.Template, language string, data map[string]any) (*RenderedTemplate, error) {
	if tmpl.ActiveVersionID == nil {
		return nil, fmt.Errorf("template %q has no active version", tmpl.DisplayName)
	}

	v, err := s.versionRepo.FindByID(*tmpl.ActiveVersionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load active version: %w", err)
	}

	l := s.resolveLocalization(v.ID, language, tmpl.DefaultLanguage)
	if l == nil {
		return nil, fmt.Errorf("no localization found for template %q", tmpl.DisplayName)
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
	// Reserved variables render as sentinels; Send substitutes the real per-message URLs once the email is known.
	return s.renderer.Render(input, WithSystemVars(data))
}

// Fallback order: requested language, base language (fr-CA to fr), template default, then "en".
func (s *Service) resolveLocalization(versionID uint, language, defaultLang string) *models.TemplateLocalization {
	if language == "" {
		language = defaultLang
	}
	if language == "" {
		language = "en"
	}

	l, err := s.localizationRepo.FindByVersionAndLanguage(versionID, language)
	if err == nil {
		return l
	}

	if idx := strings.Index(language, "-"); idx > 0 {
		base := language[:idx]
		l, err = s.localizationRepo.FindByVersionAndLanguage(versionID, base)
		if err == nil {
			return l
		}
	}

	if language != defaultLang && defaultLang != "" {
		l, err = s.localizationRepo.FindByVersionAndLanguage(versionID, defaultLang)
		if err == nil {
			return l
		}
	}

	if language != "en" && defaultLang != "en" {
		l, err = s.localizationRepo.FindByVersionAndLanguage(versionID, "en")
		if err == nil {
			return l
		}
	}

	return nil
}

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

// System headers always apply and win over the caller's custom headers on conflict.
func outboundHeaders(req *SendRequest, customEnabled bool) map[string]string {
	out := map[string]string{}
	if customEnabled {
		for k, v := range filterCustomHeaders(req.Headers) {
			out[k] = v
		}
	}
	for k, v := range req.SystemHeaders {
		for existing := range out {
			if strings.EqualFold(existing, k) {
				delete(out, existing)
			}
		}
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Date and Message-ID stay allowed so an integration can set a stable Message-ID; buildMessage only fills gaps.
func filterCustomHeaders(headers map[string]string) map[string]string {
	reserved := map[string]bool{
		"from":         true,
		"to":           true,
		"subject":      true,
		"mime-version": true,
		"content-type": true,
	}

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

func (s *Service) applyDefaultSenderName(workspaceID *uint, from string) string {
	if workspaceID == nil || s.wsSettingRepo == nil {
		return from
	}
	ws, err := s.wsSettingRepo.FindByWorkspaceID(*workspaceID)
	if err != nil || ws == nil {
		return from
	}
	return resolveSenderWithDefault(from, ws.DefaultSenderName)
}

func resolveSenderWithDefault(from, defaultName string) string {
	addr, err := mail.ParseAddress(from)
	if err != nil || addr.Name != "" {
		return from
	}
	defaultName = strings.TrimSpace(defaultName)
	if defaultName == "" {
		return from
	}
	if !isASCII(defaultName) {
		return fmt.Sprintf("%q <%s>", defaultName, addr.Address)
	}
	return (&mail.Address{Name: defaultName, Address: addr.Address}).String()
}

func (s *Service) checkDomainVerification(userID uint, workspaceID *uint, from string) error {
	if s.domainRepo == nil {
		return nil
	}

	strict := false
	if workspaceID != nil {
		if s.wsSettingRepo != nil {
			strict = s.wsSettingRepo.RequireVerifiedDomain(*workspaceID)
		}
	} else if s.userRepo != nil {
		user, err := s.userRepo.FindByID(userID)
		if err != nil {
			return fmt.Errorf("failed to load user: %w", err)
		}
		strict = user.RequireVerifiedDomain
	}

	if !strict {
		return nil
	}

	senderDomain := extractSenderDomain(from)
	if senderDomain == "" {
		return fmt.Errorf("domain_verification: could not extract domain from sender %q", from)
	}

	verified := false
	if workspaceID != nil {
		verified = s.domainRepo.IsOwnershipVerifiedInWorkspace(*workspaceID, senderDomain)
	} else {
		verified = s.domainRepo.IsOwnershipVerified(userID, senderDomain)
	}
	if !verified {
		return fmt.Errorf("domain_verification: domain %q is not verified. Add and verify the domain or disable strict domain mode", senderDomain)
	}

	return nil
}

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

func extractSenderDomain(from string) string {
	bare := extractEmail(from)
	parts := strings.SplitN(bare, "@", 2)
	if len(parts) != 2 {
		return ""
	}
	return strings.ToLower(parts[1])
}

func extractEmail(from string) string {
	addr, err := mail.ParseAddress(from)
	if err != nil {
		return from
	}
	return addr.Address
}
