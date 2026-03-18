/*
 *  MIT License
 *
 * Copyright (c) 2026 Jonas Kaninda
 *
 *  Permission is hereby granted, free of charge, to any person obtaining a copy
 *  of this software and associated documentation files (the "Software"), to deal
 *  in the Software without restriction, including without limitation the rights
 *  to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 *  copies of the Software, and to permit persons to whom the Software is
 *  furnished to do so, subject to the following conditions:
 *
 *  The above copyright notice and this permission notice shall be included in all
 *  copies or substantial portions of the Software.
 *
 *  THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 *  IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 *  FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 *  AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 *  LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 *  OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 *  SOFTWARE.
 */

package handlers

import (
	"fmt"
	"strings"

	"github.com/jkaninda/okapi"
	"github.com/jkaninda/posta/internal/models"
	"github.com/jkaninda/posta/internal/services/cache"
	"github.com/jkaninda/posta/internal/services/email"
	"github.com/jkaninda/posta/internal/services/eventbus"
	"github.com/jkaninda/posta/internal/services/settings"
	"github.com/jkaninda/posta/internal/storage/repositories"
)

type EmailHandler struct {
	service   *email.Service
	emailRepo *repositories.EmailRepository
	bus       *eventbus.EventBus
	cache     *cache.Cache
	settings  *settings.Provider
}
type SendEmailRequest struct {
	Body email.SendRequest `json:"body"`
}
type SendTemplateEmailRequest struct {
	Body email.SendTemplateRequest `json:"body"`
}
type SendBatchEmailRequest struct {
	Body email.BatchRequest `json:"body"`
}
type ListRequest struct {
	Page int `query:"page" default:"0"`
	Size int `query:"size" default:"20"`
}
type GetByIDRequest struct {
	ID int `param:"id"`
}
type GetEmailRequest struct {
	ID string `param:"id"`
}

func NewEmailHandler(service *email.Service, emailRepo *repositories.EmailRepository, bus *eventbus.EventBus, c *cache.Cache) *EmailHandler {
	return &EmailHandler{
		service:   service,
		emailRepo: emailRepo,
		bus:       bus,
		cache:     c,
	}
}

func (h *EmailHandler) SetSettings(s *settings.Provider) { h.settings = s }

// redactContent returns true if email body content should be hidden.
func (h *EmailHandler) redactContent() bool {
	if h.settings != nil && h.settings.EmailContentVisibility() {
		return false
	}
	return true
}

const redactedPlaceholder = "[redacted]"

func redactEmail(em *models.Email) {
	em.HTMLBody = redactedPlaceholder
	em.TextBody = redactedPlaceholder
}

func redactEmails(emails []models.Email) {
	for i := range emails {
		redactEmail(&emails[i])
	}
}

func (h *EmailHandler) Send(c *okapi.Context, req *SendEmailRequest) error {
	userID := c.GetInt("user_id")
	apiKeyID := c.GetInt("api_key_id")
	userEmail := c.GetString("user_email")

	resp, err := h.service.Send(c.Request().Context(), uint(userID), uint(apiKeyID), userEmail, &req.Body)
	if err != nil {
		if isRateLimitError(err) {
			return c.AbortTooManyRequests(err.Error())
		}
		if isDomainVerificationError(err) {
			return c.AbortForbidden(err.Error())
		}
		return c.AbortInternalServerError(err.Error())
	}

	// Invalidate dashboard and metrics caches after queuing
	h.cache.InvalidateUser(c.Request().Context(), userID)
	h.cache.InvalidateAnalytics(c.Request().Context())

	if h.bus != nil {
		uid := uint(userID)
		h.bus.PublishSimple(models.EventCategoryEmail, "email.queued", &uid, userEmail, c.RealIP(),
			fmt.Sprintf("Email queued to %v", req.Body.To), map[string]any{"email_uuid": resp.ID})
	}

	return ok(c, resp)
}

func (h *EmailHandler) SendWithTemplate(c *okapi.Context, req *SendTemplateEmailRequest) error {
	userID := c.GetInt("user_id")
	apiKeyID := c.GetInt("api_key_id")
	userEmail := c.GetString("user_email")

	resp, err := h.service.SendWithTemplate(c.Request().Context(), uint(userID), uint(apiKeyID), userEmail, &req.Body)
	if err != nil {
		if isRateLimitError(err) {
			return c.AbortTooManyRequests(err.Error())
		}
		if isDomainVerificationError(err) {
			return c.AbortForbidden(err.Error())
		}
		return c.AbortInternalServerError(err.Error())
	}

	h.cache.InvalidateUser(c.Request().Context(), userID)
	h.cache.InvalidateAnalytics(c.Request().Context())

	if h.bus != nil {
		uid := uint(userID)
		h.bus.PublishSimple(models.EventCategoryEmail, "email.queued", &uid, userEmail, c.RealIP(),
			fmt.Sprintf("Template email queued to %v", req.Body.To), map[string]any{"email_uuid": resp.ID, "template": req.Body.Template})
	}

	return ok(c, resp)
}

func (h *EmailHandler) SendBatch(c *okapi.Context, req *SendBatchEmailRequest) error {
	userID := c.GetInt("user_id")
	apiKeyID := c.GetInt("api_key_id")
	userEmail := c.GetString("user_email")

	resp, err := h.service.SendBatch(c.Request().Context(), uint(userID), uint(apiKeyID), userEmail, &req.Body)
	if err != nil {
		if isRateLimitError(err) {
			return c.AbortTooManyRequests(err.Error())
		}
		if isDomainVerificationError(err) {
			return c.AbortForbidden(err.Error())
		}
		return c.AbortInternalServerError(err.Error())
	}

	h.cache.InvalidateUser(c.Request().Context(), userID)
	h.cache.InvalidateAnalytics(c.Request().Context())

	if h.bus != nil {
		uid := uint(userID)
		h.bus.PublishSimple(models.EventCategoryEmail, "email.batch_queued", &uid, userEmail, c.RealIP(),
			fmt.Sprintf("Batch of %d emails queued", len(resp.Results)), map[string]any{"count": len(resp.Results)})
	}

	return ok(c, resp)
}

func normalizePageParams(page, size int) (int, int, int) {
	if size <= 0 || size > 100 {
		size = 20
	}
	if page < 0 {
		page = 0
	}
	offset := page * size
	return page, size, offset
}

func (h *EmailHandler) List(c *okapi.Context, req *ListRequest) error {
	userID := c.GetInt("user_id")
	page, size, offset := normalizePageParams(req.Page, req.Size)

	emails, total, err := h.emailRepo.FindByUserID(uint(userID), size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list emails")
	}

	if h.redactContent() {
		redactEmails(emails)
	}

	return paginated(c, emails, total, page, size)
}

func (h *EmailHandler) Get(c *okapi.Context, req *GetEmailRequest) error {
	em, err := h.emailRepo.FindByUUID(req.ID)
	if err != nil {
		return c.AbortNotFound("email not found")
	}

	userID := c.GetInt("user_id")
	if em.UserID != uint(userID) {
		return c.AbortNotFound("email not found")
	}

	if h.redactContent() {
		redactEmail(em)
	}

	return ok(c, em)
}

// EmailStatusResponse is a lightweight view of an email's delivery status.
type EmailStatusResponse struct {
	ID           string             `json:"id"`
	Status       models.EmailStatus `json:"status"`
	ErrorMessage string             `json:"error_message,omitempty"`
	RetryCount   int                `json:"retry_count"`
	CreatedAt    string             `json:"created_at"`
	SentAt       *string            `json:"sent_at,omitempty"`
}

// GetStatus returns only the delivery status of an email (owner only).
func (h *EmailHandler) GetStatus(c *okapi.Context, req *GetEmailRequest) error {
	em, err := h.emailRepo.FindByUUID(req.ID)
	if err != nil {
		return c.AbortNotFound("email not found")
	}

	userID := c.GetInt("user_id")
	if em.UserID != uint(userID) {
		return c.AbortNotFound("email not found")
	}

	resp := EmailStatusResponse{
		ID:           em.UUID,
		Status:       em.Status,
		ErrorMessage: em.ErrorMessage,
		RetryCount:   em.RetryCount,
		CreatedAt:    em.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if em.SentAt != nil {
		s := em.SentAt.Format("2006-01-02T15:04:05Z07:00")
		resp.SentAt = &s
	}

	return ok(c, resp)
}

// Retry re-enqueues a failed email for another delivery attempt.
func (h *EmailHandler) Retry(c *okapi.Context, req *GetEmailRequest) error {
	userID := c.GetInt("user_id")
	resp, err := h.service.RetryEmail(req.ID, uint(userID))
	if err != nil {
		if err.Error() == "email not found" {
			return c.AbortNotFound(err.Error())
		}
		return c.AbortBadRequest(err.Error())
	}
	return ok(c, resp)
}

// Dev mode handlers
func (h *EmailHandler) DevList(c *okapi.Context, req *ListRequest) error {
	page, size, offset := normalizePageParams(req.Page, req.Size)

	emails, total, err := h.emailRepo.FindAll(size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list emails")
	}

	return paginated(c, emails, total, page, size)
}

func (h *EmailHandler) DevGet(c *okapi.Context) error {
	uuid := c.PathParam("id")
	if uuid == "" {
		return c.AbortBadRequest("invalid email id")
	}

	em, err := h.emailRepo.FindByUUID(uuid)
	if err != nil {
		return c.AbortNotFound("email not found")
	}

	return ok(c, em)
}

func isRateLimitError(err error) bool {
	return err != nil && len(err.Error()) > 11 && err.Error()[:11] == "rate_limit:"
}

func isDomainVerificationError(err error) bool {
	return err != nil && strings.HasPrefix(err.Error(), "domain_verification:")
}
