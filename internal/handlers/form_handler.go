// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package handlers

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/audit"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
)

const metaSlug = "slug"

const publicKeyAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
const publicKeyLength = 22

var formSlugPattern = regexp.MustCompile(`[^a-z0-9]+`)

type FormHandler struct {
	repo        *repositories.FormRepository
	messageRepo *repositories.MessageRepository
	domainRepo  *repositories.DomainRepository
	audit       *audit.Logger
	apiBaseURL  string
}

func NewFormHandler(
	repo *repositories.FormRepository,
	messageRepo *repositories.MessageRepository,
	domainRepo *repositories.DomainRepository,
	auditLogger *audit.Logger,
	apiBaseURL string,
) *FormHandler {
	return &FormHandler{
		repo:        repo,
		messageRepo: messageRepo,
		domainRepo:  domainRepo,
		audit:       auditLogger,
		apiBaseURL:  strings.TrimSuffix(apiBaseURL, "/"),
	}
}

type CreateFormRequest struct {
	Body struct {
		Name             string   `json:"name" required:"true"`
		Slug             string   `json:"slug"`
		Description      string   `json:"description"`
		AllowedOrigins   []string `json:"allowed_origins"`
		StrictOrigin     bool     `json:"strict_origin"`
		RedirectURL      string   `json:"redirect_url"`
		AllowAttachments bool     `json:"allow_attachments"`
		RequireNonce     bool     `json:"require_nonce"`
		NotifyEmails     []string `json:"notify_emails"`
		NotifyMode       string   `json:"notify_mode" enum:"immediate,hourly,daily,off"`
		ReplyFrom        string   `json:"reply_from"`
		ReplyFromName    string   `json:"reply_from_name"`
	} `json:"body"`
}

type UpdateFormRequest struct {
	ID   int `param:"id"`
	Body struct {
		Name                string    `json:"name"`
		Slug                string    `json:"slug"`
		Description         string    `json:"description"`
		Status              string    `json:"status" enum:"active,paused,archived"`
		AllowedOrigins      *[]string `json:"allowed_origins"`
		StrictOrigin        *bool     `json:"strict_origin"`
		RedirectURL         *string   `json:"redirect_url"`
		MaxBodyBytes        *int64    `json:"max_body_bytes"`
		MaxFields           *int      `json:"max_fields"`
		AllowAttachments    *bool     `json:"allow_attachments"`
		HoneypotField       *string   `json:"honeypot_field"`
		RequireNonce        *bool     `json:"require_nonce"`
		MinFillSeconds      *int      `json:"min_fill_seconds"`
		ScanEnabled         *bool     `json:"scan_enabled"`
		FlagThreshold       *float64  `json:"flag_threshold"`
		QuarantineThreshold *float64  `json:"quarantine_threshold"`
		RejectThreshold     *float64  `json:"reject_threshold"`
		NotifyEnabled       *bool     `json:"notify_enabled"`
		NotifyEmails        *[]string `json:"notify_emails"`
		NotifyMode          *string   `json:"notify_mode" doc:"One of: immediate, hourly, daily, off"`
		NotifyOnFlagged     *bool     `json:"notify_on_flagged"`
		ReplyFrom           *string   `json:"reply_from"`
		ReplyFromName       *string   `json:"reply_from_name"`
		RetentionDays       *int      `json:"retention_days"`
	} `json:"body"`
}

type FormIDRequest struct {
	ID int `param:"id"`
}

type FormSnippetResponse struct {
	Endpoint  string `json:"endpoint"`
	PublicKey string `json:"public_key"`
	HTML      string `json:"html"`
	Fetch     string `json:"fetch"`
}

func (h *FormHandler) Create(c *okapi.Context, req *CreateFormRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	scope := getScope(c)
	if scope.WorkspaceID == nil {
		return c.AbortBadRequest("a workspace is required to create a form")
	}

	name := strings.TrimSpace(req.Body.Name)
	if name == "" {
		return c.AbortBadRequest("name is required")
	}

	slug := formSlug(req.Body.Slug)
	if slug == "" {
		slug = formSlug(name)
	}
	if slug == "" {
		return c.AbortBadRequest("could not derive a slug from the name")
	}
	if h.repo.SlugExists(*scope.WorkspaceID, slug, 0) {
		return c.AbortConflict("a form with that slug already exists")
	}

	origins, err := normalizeOrigins(req.Body.AllowedOrigins)
	if err != nil {
		return c.AbortBadRequest(err.Error())
	}
	notify, err := normalizeEmails(req.Body.NotifyEmails)
	if err != nil {
		return c.AbortBadRequest(err.Error())
	}
	if req.Body.RedirectURL != "" && !isHTTPURL(req.Body.RedirectURL) {
		return c.AbortBadRequest("redirect_url must be an absolute http(s) URL")
	}
	replyFrom := strings.TrimSpace(req.Body.ReplyFrom)
	if replyFrom != "" {
		if err := h.checkSender(scope, replyFrom); err != nil {
			return c.AbortBadRequest(err.Error())
		}
	}

	key, err := generatePublicKey()
	if err != nil {
		return c.AbortInternalServerError("failed to generate form key")
	}

	userID := uint(c.GetInt("user_id"))
	form := &models.Form{
		WorkspaceID:         scope.WorkspaceID,
		Name:                name,
		Slug:                slug,
		Description:         strings.TrimSpace(req.Body.Description),
		PublicKey:           key,
		Status:              models.FormStatusActive,
		AllowedOrigins:      origins,
		StrictOrigin:        req.Body.StrictOrigin,
		RedirectURL:         strings.TrimSpace(req.Body.RedirectURL),
		MaxBodyBytes:        models.DefaultMaxBodyBytes,
		MaxFields:           models.DefaultMaxFields,
		AllowAttachments:    req.Body.AllowAttachments,
		HoneypotField:       models.DefaultHoneypotField,
		RequireNonce:        req.Body.RequireNonce,
		MinFillSeconds:      3,
		ScanEnabled:         true,
		FlagThreshold:       3,
		QuarantineThreshold: 6,
		RejectThreshold:     10,
		NotifyEnabled:       true,
		NotifyEmails:        notify,
		NotifyMode:          normalizeNotifyMode(req.Body.NotifyMode),
		NotifyOnFlagged:     true,
		ReplyFrom:           replyFrom,
		ReplyFromName:       strings.TrimSpace(req.Body.ReplyFromName),
		LastEditedByID:      &userID,
	}

	if err := h.repo.Create(form); err != nil {
		return c.AbortConflict("failed to create form")
	}

	h.log(c, "form.created", fmt.Sprintf("Form %s created", form.Name), map[string]any{
		metaFormID: form.ID, metaSlug: form.Slug,
	})
	return created(c, form)
}

func (h *FormHandler) List(c *okapi.Context, req *ListRequest) error {
	page, size, offset := normalizePageParams(req.Page, req.Size)
	forms, total, err := h.repo.FindByScope(getScope(c), size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list forms")
	}
	return paginated(c, forms, total, page, size)
}

func (h *FormHandler) Get(c *okapi.Context, req *FormIDRequest) error {
	form, err := h.repo.FindByIDForScope(getScope(c), uint(req.ID))
	if err != nil {
		return c.AbortNotFound("form not found")
	}
	return ok(c, form)
}

func (h *FormHandler) Update(c *okapi.Context, req *UpdateFormRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	scope := getScope(c)
	form, err := h.repo.FindByIDForScope(scope, uint(req.ID))
	if err != nil {
		return c.AbortNotFound("form not found")
	}

	b := req.Body
	if strings.TrimSpace(b.Name) != "" {
		form.Name = strings.TrimSpace(b.Name)
	}
	if strings.TrimSpace(b.Slug) != "" {
		slug := formSlug(b.Slug)
		if slug == "" {
			return c.AbortBadRequest("invalid slug")
		}
		if h.repo.SlugExists(*scope.WorkspaceID, slug, form.ID) {
			return c.AbortConflict("a form with that slug already exists")
		}
		form.Slug = slug
	}
	if strings.TrimSpace(b.Description) != "" {
		form.Description = strings.TrimSpace(b.Description)
	}
	if b.Status != "" {
		status := models.FormStatus(b.Status)
		if status != models.FormStatusActive && status != models.FormStatusPaused && status != models.FormStatusArchived {
			return c.AbortBadRequest("invalid status")
		}
		form.Status = status
	}
	if b.AllowedOrigins != nil {
		origins, err := normalizeOrigins(*b.AllowedOrigins)
		if err != nil {
			return c.AbortBadRequest(err.Error())
		}
		form.AllowedOrigins = origins
	}
	if b.StrictOrigin != nil {
		form.StrictOrigin = *b.StrictOrigin
	}
	if b.RedirectURL != nil {
		target := strings.TrimSpace(*b.RedirectURL)
		if target != "" && !isHTTPURL(target) {
			return c.AbortBadRequest("redirect_url must be an absolute http(s) URL")
		}
		form.RedirectURL = target
	}
	if b.MaxBodyBytes != nil && *b.MaxBodyBytes > 0 {
		form.MaxBodyBytes = clampInt64(*b.MaxBodyBytes, 1024, 1048576)
	}
	if b.MaxFields != nil && *b.MaxFields > 0 {
		form.MaxFields = clampInt(*b.MaxFields, 1, 200)
	}
	if b.AllowAttachments != nil {
		form.AllowAttachments = *b.AllowAttachments
	}
	if b.HoneypotField != nil {
		field := strings.TrimSpace(*b.HoneypotField)
		if field == "" {
			field = models.DefaultHoneypotField
		}
		form.HoneypotField = field
	}
	if b.RequireNonce != nil {
		form.RequireNonce = *b.RequireNonce
	}
	if b.MinFillSeconds != nil {
		form.MinFillSeconds = clampInt(*b.MinFillSeconds, 0, 120)
	}
	if b.ScanEnabled != nil {
		form.ScanEnabled = *b.ScanEnabled
	}
	if b.FlagThreshold != nil {
		form.FlagThreshold = *b.FlagThreshold
	}
	if b.QuarantineThreshold != nil {
		form.QuarantineThreshold = *b.QuarantineThreshold
	}
	if b.RejectThreshold != nil {
		form.RejectThreshold = *b.RejectThreshold
	}
	if form.QuarantineThreshold < form.FlagThreshold || form.RejectThreshold < form.QuarantineThreshold {
		return c.AbortBadRequest("thresholds must satisfy flag <= quarantine <= reject")
	}
	if b.NotifyEnabled != nil {
		form.NotifyEnabled = *b.NotifyEnabled
	}
	if b.NotifyEmails != nil {
		notify, err := normalizeEmails(*b.NotifyEmails)
		if err != nil {
			return c.AbortBadRequest(err.Error())
		}
		form.NotifyEmails = notify
	}
	if b.NotifyMode != nil {
		mode, valid := parseNotifyMode(*b.NotifyMode)
		if !valid {
			return c.AbortBadRequest("notify_mode must be one of: immediate, hourly, daily, off")
		}
		form.NotifyMode = mode
	}
	if b.NotifyOnFlagged != nil {
		form.NotifyOnFlagged = *b.NotifyOnFlagged
	}
	if b.ReplyFrom != nil {
		replyFrom := strings.TrimSpace(*b.ReplyFrom)
		if replyFrom != "" {
			if err := h.checkSender(scope, replyFrom); err != nil {
				return c.AbortBadRequest(err.Error())
			}
		}
		form.ReplyFrom = replyFrom
	}
	if b.ReplyFromName != nil {
		form.ReplyFromName = strings.TrimSpace(*b.ReplyFromName)
	}
	if b.RetentionDays != nil {
		form.RetentionDays = clampInt(*b.RetentionDays, 0, 3650)
	}

	userID := uint(c.GetInt("user_id"))
	now := time.Now()
	form.LastEditedByID = &userID
	form.UpdatedAt = &now

	if err := h.repo.Update(form); err != nil {
		return c.AbortInternalServerError("failed to update form")
	}

	h.log(c, "form.updated", fmt.Sprintf("Form %s updated", form.Name), map[string]any{metaFormID: form.ID})
	return ok(c, form)
}

func (h *FormHandler) Delete(c *okapi.Context, req *FormIDRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	form, err := h.repo.FindByIDForScope(getScope(c), uint(req.ID))
	if err != nil {
		return c.AbortNotFound("form not found")
	}
	if err := h.repo.Delete(form.ID); err != nil {
		return c.AbortInternalServerError("failed to delete form")
	}
	h.log(c, "form.deleted", fmt.Sprintf("Form %s deleted", form.Name), map[string]any{metaFormID: form.ID})
	return noContent(c)
}

func (h *FormHandler) RotateKey(c *okapi.Context, req *FormIDRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	form, err := h.repo.FindByIDForScope(getScope(c), uint(req.ID))
	if err != nil {
		return c.AbortNotFound("form not found")
	}
	key, err := generatePublicKey()
	if err != nil {
		return c.AbortInternalServerError("failed to generate form key")
	}
	now := time.Now()
	form.PublicKey = key
	form.UpdatedAt = &now
	if err := h.repo.Update(form); err != nil {
		return c.AbortInternalServerError("failed to rotate form key")
	}
	h.log(c, "form.key_rotated", fmt.Sprintf("Form %s key rotated", form.Name), map[string]any{metaFormID: form.ID})
	return ok(c, form)
}

func (h *FormHandler) Snippet(c *okapi.Context, req *FormIDRequest) error {
	form, err := h.repo.FindByIDForScope(getScope(c), uint(req.ID))
	if err != nil {
		return c.AbortNotFound("form not found")
	}
	endpoint := fmt.Sprintf("%s/api/v1/f/%s", h.apiBaseURL, form.PublicKey)
	return ok(c, FormSnippetResponse{
		Endpoint:  endpoint,
		PublicKey: form.PublicKey,
		HTML:      buildHTMLSnippet(endpoint, form),
		Fetch:     buildFetchSnippet(endpoint),
	})
}

func (h *FormHandler) checkSender(scope repositories.ResourceScope, address string) error {
	addr, err := mail.ParseAddress(address)
	if err != nil {
		return fmt.Errorf("reply_from must be a valid email address")
	}
	if h.domainRepo == nil {
		return nil
	}
	at := strings.LastIndex(addr.Address, "@")
	if at < 0 {
		return fmt.Errorf("reply_from must be a valid email address")
	}
	domain := strings.ToLower(addr.Address[at+1:])

	domains, _, err := h.domainRepo.FindByScope(scope, 200, 0)
	if err != nil {
		return nil
	}
	for _, d := range domains {
		if strings.EqualFold(d.Domain, domain) && d.IsOwnershipVerified() {
			return nil
		}
	}
	return fmt.Errorf("reply_from domain %s is not a verified domain in this workspace", domain)
}

func (h *FormHandler) log(c *okapi.Context, action, message string, meta map[string]any) {
	if h.audit == nil {
		return
	}
	h.audit.LogCtx(c, action, message, meta)
}

func formSlug(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	v = formSlugPattern.ReplaceAllString(v, "-")
	v = strings.Trim(v, "-")
	if len(v) > 60 {
		v = strings.Trim(v[:60], "-")
	}
	return v
}

func generatePublicKey() (string, error) {
	out := make([]byte, publicKeyLength)
	max := big.NewInt(int64(len(publicKeyAlphabet)))
	for i := range out {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		out[i] = publicKeyAlphabet[n.Int64()]
	}
	return string(out), nil
}

func normalizeOrigins(in []string) ([]string, error) {
	out := make([]string, 0, len(in))
	for _, raw := range in {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if raw == "*" {
			out = append(out, raw)
			continue
		}
		u, err := url.Parse(raw)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return nil, fmt.Errorf("allowed origin %q must include a scheme and host, e.g. https://example.com", raw)
		}
		out = append(out, strings.ToLower(u.Scheme+"://"+u.Host))
	}
	return out, nil
}

func normalizeEmails(in []string) ([]string, error) {
	out := make([]string, 0, len(in))
	for _, raw := range in {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		addr, err := mail.ParseAddress(raw)
		if err != nil {
			return nil, fmt.Errorf("%q is not a valid email address", raw)
		}
		out = append(out, addr.Address)
	}
	if len(out) > 10 {
		return nil, fmt.Errorf("at most 10 notification recipients are allowed")
	}
	return out, nil
}

func parseNotifyMode(v string) (models.NotifyMode, bool) {
	switch models.NotifyMode(strings.ToLower(strings.TrimSpace(v))) {
	case models.NotifyModeImmediate:
		return models.NotifyModeImmediate, true
	case models.NotifyModeHourly:
		return models.NotifyModeHourly, true
	case models.NotifyModeDaily:
		return models.NotifyModeDaily, true
	case models.NotifyModeOff:
		return models.NotifyModeOff, true
	default:
		return models.NotifyModeImmediate, false
	}
}

func normalizeNotifyMode(v string) models.NotifyMode {
	if strings.TrimSpace(v) == "" {
		return models.NotifyModeImmediate
	}
	mode, _ := parseNotifyMode(v)
	return mode
}

func isHTTPURL(v string) bool {
	u, err := url.Parse(v)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampInt64(v, lo, hi int64) int64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func buildHTMLSnippet(endpoint string, form *models.Form) string {
	return fmt.Sprintf(`<form action="%s" method="POST">
  <label for="posta-name">Name</label>
  <input id="posta-name" type="text" name="name" required>

  <label for="posta-email">Email</label>
  <input id="posta-email" type="email" name="email" required>

  <label for="posta-message">Message</label>
  <textarea id="posta-message" name="message" required></textarea>

  <div style="position:absolute;left:-9999px" aria-hidden="true">
    <input type="text" name="%s" tabindex="-1" autocomplete="off">
  </div>

  <button type="submit">Send</button>
</form>`, endpoint, form.Honeypot())
}

func buildFetchSnippet(endpoint string) string {
	return fmt.Sprintf(`await fetch(%q, {
  method: 'POST',
  headers: { 'Content-Type': 'text/plain;charset=UTF-8' },
  body: JSON.stringify({
    name: form.name.value,
    email: form.email.value,
    message: form.message.value,
  }),
})`, endpoint)
}
