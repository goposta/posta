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
	"fmt"
	"os"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jkaninda/okapi"
	"github.com/jkaninda/posta/internal/models"
	"github.com/jkaninda/posta/internal/services/cache"
	"github.com/jkaninda/posta/internal/services/eventbus"
	"github.com/jkaninda/posta/internal/services/seeder"
	"github.com/jkaninda/posta/internal/services/settings"
	"github.com/jkaninda/posta/internal/storage/repositories"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AdminHandler struct {
	db             *gorm.DB
	cache          *cache.Cache
	userRepo       *repositories.UserRepository
	keyRepo        *repositories.APIKeyRepository
	emailRepo      *repositories.EmailRepository
	whDeliveryRepo *repositories.WebhookDeliveryRepository
	inspector      *asynq.Inspector
	bus            *eventbus.EventBus
	seeder         *seeder.Seeder
	embeddedWorker bool
	emailSettings  *settings.Provider
}
type AdminCreateUserRequest struct {
	Body struct {
		Name     string `json:"name"`
		Email    string `json:"email" required:"true" format:"email"`
		Password string `json:"password" required:"true" minLength:"8"`
		Role     string `json:"role" enum:"admin,user" required:"true"`
	} `json:"body"`
}
type AdminUpdateUserRequest struct {
	ID   int `param:"id"`
	Body struct {
		Role   string `json:"role" enum:"admin,user"`
		Active *bool  `json:"active"`
	} `json:"body"`
}
type AdminDeleteUserRequest struct {
	ID int `param:"id"`
}

// PlatformMetrics holds aggregate platform metrics.
type PlatformMetrics struct {
	TotalUsers        int64                              `json:"total_users"`
	TotalEmails       int64                              `json:"total_emails"`
	QueuedEmails      int64                              `json:"queued_emails"`
	ProcessingEmails  int64                              `json:"processing_emails"`
	SentEmails        int64                              `json:"sent_emails"`
	FailedEmails      int64                              `json:"failed_emails"`
	SuppressedEmails  int64                              `json:"suppressed_emails"`
	FailureRate       float64                            `json:"failure_rate"`
	TotalAPIKeys      int64                              `json:"total_api_keys"`
	ActiveAPIKeys     int64                              `json:"active_api_keys"`
	TotalBounces      int64                              `json:"total_bounces"`
	TotalSuppressions int64                              `json:"total_suppressions"`
	ActiveWorkers     int                                `json:"active_workers"`
	SharedSmtpServers int64                              `json:"shared_smtp_servers"`
	TotalDomains      int64                              `json:"total_domains"`
	WebhookDeliveries *repositories.WebhookDeliveryStats `json:"webhook_deliveries"`
}

type AdminRevokeKeyRequest struct {
	ID int `param:"id"`
}

type AdminGetUserRequest struct {
	ID int `param:"id"`
}

// UserDetailMetrics holds per-user metrics.
type UserDetailMetrics struct {
	User              *models.User                       `json:"user"`
	TotalEmails       int64                              `json:"total_emails"`
	SentEmails        int64                              `json:"sent_emails"`
	FailedEmails      int64                              `json:"failed_emails"`
	SuppressedEmails  int64                              `json:"suppressed_emails"`
	FailureRate       float64                            `json:"failure_rate"`
	TotalAPIKeys      int64                              `json:"total_api_keys"`
	ActiveAPIKeys     int64                              `json:"active_api_keys"`
	TotalContacts     int64                              `json:"total_contacts"`
	TotalBounces      int64                              `json:"total_bounces"`
	TotalSuppressions int64                              `json:"total_suppressions"`
	TotalDomains      int64                              `json:"total_domains"`
	TotalSmtpServers  int64                              `json:"total_smtp_servers"`
	WebhookDeliveries *repositories.WebhookDeliveryStats `json:"webhook_deliveries"`
}

func NewAdminHandler(db *gorm.DB, c *cache.Cache, userRepo *repositories.UserRepository, keyRepo *repositories.APIKeyRepository, emailRepo *repositories.EmailRepository, whDeliveryRepo *repositories.WebhookDeliveryRepository, inspector *asynq.Inspector, bus *eventbus.EventBus, seeder *seeder.Seeder, embeddedWorker bool) *AdminHandler {
	return &AdminHandler{db: db, cache: c, userRepo: userRepo, keyRepo: keyRepo, emailRepo: emailRepo, whDeliveryRepo: whDeliveryRepo, inspector: inspector, bus: bus, seeder: seeder, embeddedWorker: embeddedWorker}
}

func (h *AdminHandler) SetEmailSettings(s *settings.Provider) {
	h.emailSettings = s
}

// CreateUser allows admins to create a new user.
func (h *AdminHandler) CreateUser(c *okapi.Context, req *AdminCreateUserRequest) error {
	role := models.UserRole(req.Body.Role)
	if role != models.UserRoleAdmin && role != models.UserRoleUser {
		role = models.UserRoleUser
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Body.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.AbortInternalServerError("failed to hash password", err)
	}

	user := &models.User{
		Name:         req.Body.Name,
		Email:        req.Body.Email,
		PasswordHash: string(hash),
		Role:         role,
	}

	if err := h.userRepo.Create(user); err != nil {
		return c.AbortConflict("email already registered")
	}

	// Seed default templates, stylesheets, and languages for the new user
	if h.seeder != nil {
		go h.seeder.SeedUserDefaults(user.ID, user.Name)
	}

	if h.bus != nil {
		adminID := uint(c.GetInt("user_id"))
		h.bus.PublishSimple(models.EventCategoryUser, "user.created", &adminID, c.GetString("email"), c.RealIP(),
			fmt.Sprintf("User %q created", user.Email), map[string]any{"user_id": user.ID, "role": string(user.Role)})
	}

	return created(c, user)
}

// ListUsers returns all users.
func (h *AdminHandler) ListUsers(c *okapi.Context, req *ListRequest) error {
	page, size, offset := normalizePageParams(req.Page, req.Size)

	users, total, err := h.userRepo.FindAll(size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list users")
	}

	return paginated(c, users, total, page, size)
}

// UpdateUser allows admins to change a user's role and active status.
func (h *AdminHandler) UpdateUser(c *okapi.Context, req *AdminUpdateUserRequest) error {
	user, err := h.userRepo.FindByID(uint(req.ID))
	if err != nil {
		return c.AbortNotFound("user not found")
	}

	if req.Body.Role != "" {
		user.Role = models.UserRole(req.Body.Role)
	}
	if req.Body.Active != nil {
		if !*req.Body.Active && user.ID == uint(c.GetInt("user_id")) {
			return c.AbortBadRequest("cannot disable your own account")
		}
		user.Active = *req.Body.Active
	}
	if err := h.userRepo.Update(user); err != nil {
		return c.AbortInternalServerError("failed to update user")
	}

	if h.bus != nil {
		adminID := uint(c.GetInt("user_id"))
		h.bus.PublishSimple(models.EventCategoryUser, "user.updated", &adminID, c.GetString("email"), c.RealIP(),
			fmt.Sprintf("User %q updated", user.Email), map[string]any{"user_id": user.ID})
	}

	return ok(c, user)
}

// DeleteUser deletes a user (admin only).
func (h *AdminHandler) DeleteUser(c *okapi.Context, req *AdminDeleteUserRequest) error {
	currentUserID := c.GetInt("user_id")
	if req.ID == currentUserID {
		return c.AbortBadRequest("cannot delete your own account")
	}

	_, err := h.userRepo.FindByID(uint(req.ID))
	if err != nil {
		return c.AbortNotFound("user not found")
	}

	if err := h.userRepo.Delete(uint(req.ID)); err != nil {
		return c.AbortInternalServerError("failed to delete user")
	}

	if h.bus != nil {
		adminID := uint(c.GetInt("user_id"))
		h.bus.PublishSimple(models.EventCategoryUser, "user.deleted", &adminID, c.GetString("email"), c.RealIP(),
			fmt.Sprintf("User ID %d deleted", req.ID), map[string]any{"deleted_user_id": req.ID})
	}

	return noContent(c)
}

// ListAllEmails returns all emails across all users (admin only).
func (h *AdminHandler) ListAllEmails(c *okapi.Context, req *ListRequest) error {
	page, size, offset := normalizePageParams(req.Page, req.Size)

	emails, total, err := h.emailRepo.FindAll(size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list emails")
	}

	if h.emailSettings == nil || !h.emailSettings.EmailContentVisibility() {
		redactEmails(emails)
	}

	return paginated(c, emails, total, page, size)
}

// ListAllAPIKeys returns all API keys across all users (admin only).
func (h *AdminHandler) ListAllAPIKeys(c *okapi.Context, req *ListRequest) error {
	page, size, offset := normalizePageParams(req.Page, req.Size)

	keys, total, err := h.keyRepo.FindAll(size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list API keys")
	}

	return paginated(c, keys, total, page, size)
}

// RevokeAPIKey allows admins to revoke any API key.
func (h *AdminHandler) RevokeAPIKey(c *okapi.Context, req *AdminRevokeKeyRequest) error {
	_, err := h.keyRepo.FindByID(uint(req.ID))
	if err != nil {
		return c.AbortNotFound("API key not found")
	}

	if err := h.keyRepo.Revoke(uint(req.ID)); err != nil {
		return c.AbortInternalServerError("failed to revoke API key")
	}

	if h.bus != nil {
		adminID := uint(c.GetInt("user_id"))
		h.bus.PublishSimple(models.EventCategoryUser, "apikey.revoked", &adminID, c.GetString("email"), c.RealIP(),
			fmt.Sprintf("API key ID %d revoked", req.ID), map[string]any{"api_key_id": req.ID})
	}

	return ok(c, okapi.M{"message": "API key revoked"})
}

// Metrics returns platform-wide metrics (admin only).
func (h *AdminHandler) Metrics(c *okapi.Context) error {
	ctx := c.Request().Context()

	// Try cache first
	cacheKey := cache.AdminMetricsKey()
	var m PlatformMetrics
	if h.cache.Get(ctx, cacheKey, &m) {
		// Always fetch live worker count since it's cheap and real-time.
		if h.inspector != nil {
			if servers, err := h.inspector.Servers(); err == nil {
				m.ActiveWorkers = len(servers)
			}
		}
		return ok(c, m)
	}

	h.db.Model(&models.User{}).Count(&m.TotalUsers)
	h.db.Model(&models.Email{}).Count(&m.TotalEmails)
	h.db.Model(&models.Email{}).Where("status = ?", models.EmailStatusQueued).Count(&m.QueuedEmails)
	h.db.Model(&models.Email{}).Where("status = ?", models.EmailStatusProcessing).Count(&m.ProcessingEmails)
	h.db.Model(&models.Email{}).Where("status = ?", models.EmailStatusSent).Count(&m.SentEmails)
	h.db.Model(&models.Email{}).Where("status = ?", models.EmailStatusFailed).Count(&m.FailedEmails)
	h.db.Model(&models.Email{}).Where("status = ?", models.EmailStatusSuppressed).Count(&m.SuppressedEmails)
	h.db.Model(&models.APIKey{}).Count(&m.TotalAPIKeys)
	h.db.Model(&models.APIKey{}).Where("revoked = false").Count(&m.ActiveAPIKeys)
	h.db.Model(&models.Bounce{}).Count(&m.TotalBounces)
	h.db.Model(&models.Suppression{}).Count(&m.TotalSuppressions)
	h.db.Model(&models.Server{}).Count(&m.SharedSmtpServers)
	h.db.Model(&models.Domain{}).Count(&m.TotalDomains)

	if m.TotalEmails > 0 {
		m.FailureRate = float64(m.FailedEmails) / float64(m.TotalEmails) * 100
	}

	if h.inspector != nil {
		if servers, err := h.inspector.Servers(); err == nil {
			m.ActiveWorkers = len(servers)
		}
	}

	// Webhook delivery stats (platform-wide)
	if whStats, err := h.whDeliveryRepo.StatsAll(); err == nil {
		m.WebhookDeliveries = whStats
	}

	h.cache.Set(ctx, cacheKey, m, cache.AdminMetricsTTL)

	return ok(c, m)
}

// UserMetrics returns detailed metrics for a specific user (admin only).
func (h *AdminHandler) UserMetrics(c *okapi.Context, req *AdminGetUserRequest) error {
	ctx := c.Request().Context()

	// Try cache first
	cacheKey := cache.UserMetricsKey(req.ID)
	var m UserDetailMetrics
	if h.cache.Get(ctx, cacheKey, &m) {
		return ok(c, m)
	}

	user, err := h.userRepo.FindByID(uint(req.ID))
	if err != nil {
		return c.AbortNotFound("user not found")
	}

	m.User = user

	h.db.Model(&models.Email{}).Where("user_id = ?", req.ID).Count(&m.TotalEmails)
	h.db.Model(&models.Email{}).Where("user_id = ? AND status = ?", req.ID, models.EmailStatusSent).Count(&m.SentEmails)
	h.db.Model(&models.Email{}).Where("user_id = ? AND status = ?", req.ID, models.EmailStatusFailed).Count(&m.FailedEmails)
	h.db.Model(&models.Email{}).Where("user_id = ? AND status = ?", req.ID, models.EmailStatusSuppressed).Count(&m.SuppressedEmails)
	h.db.Model(&models.APIKey{}).Where("user_id = ?", req.ID).Count(&m.TotalAPIKeys)
	h.db.Model(&models.APIKey{}).Where("user_id = ? AND revoked = false", req.ID).Count(&m.ActiveAPIKeys)
	h.db.Model(&models.Contact{}).Where("user_id = ?", req.ID).Count(&m.TotalContacts)
	h.db.Model(&models.Bounce{}).Where("user_id = ?", req.ID).Count(&m.TotalBounces)
	h.db.Model(&models.Suppression{}).Where("user_id = ?", req.ID).Count(&m.TotalSuppressions)
	h.db.Model(&models.Domain{}).Where("user_id = ?", req.ID).Count(&m.TotalDomains)
	h.db.Model(&models.SMTPServer{}).Where("user_id = ?", req.ID).Count(&m.TotalSmtpServers)

	if m.TotalEmails > 0 {
		m.FailureRate = float64(m.FailedEmails) / float64(m.TotalEmails) * 100
	}

	// Webhook delivery stats for this user
	if whStats, err := h.whDeliveryRepo.StatsByUserID(uint(req.ID)); err == nil {
		m.WebhookDeliveries = whStats
	}

	h.cache.Set(ctx, cacheKey, m, cache.UserMetricsTTL)

	return ok(c, m)
}

// Disable2FA allows admins to disable 2FA for a user.
func (h *AdminHandler) Disable2FA(c *okapi.Context, req *AdminGetUserRequest) error {
	user, err := h.userRepo.FindByID(uint(req.ID))
	if err != nil {
		return c.AbortNotFound("user not found")
	}

	if !user.TwoFactorEnabled {
		return c.AbortBadRequest("2FA is not enabled for this user")
	}

	user.TwoFactorEnabled = false
	user.TwoFactorSecret = ""
	if err := h.userRepo.Update(user); err != nil {
		return c.AbortInternalServerError("failed to disable 2FA")
	}

	if h.bus != nil {
		adminID := uint(c.GetInt("user_id"))
		h.bus.PublishSimple(models.EventCategoryUser, "user.2fa_disabled", &adminID, c.GetString("email"), c.RealIP(),
			fmt.Sprintf("2FA disabled for user %q by admin", user.Email), map[string]any{"user_id": user.ID})
	}

	return ok(c, okapi.M{"message": "2FA disabled"})
}

// WorkerStatus is sent over SSE with the current worker count and details.
type WorkerStatus struct {
	ActiveWorkers int            `json:"active_workers"`
	Workers       []WorkerDetail `json:"workers"`
}

// WorkerDetail holds info about a single connected worker.
type WorkerDetail struct {
	Host   string         `json:"host"`
	PID    int            `json:"pid"`
	Queues map[string]int `json:"queues"`
	Type   string         `json:"type"` // "embedded" or "standalone"
}

// WorkerStream sends real-time worker status updates via SSE.
func (h *AdminHandler) WorkerStream(c *okapi.Context) error {
	ctx := c.Request().Context()

	w := c.ResponseWriter()

	sendStatus := func() error {
		status := h.buildWorkerStatus()
		msg := okapi.Message{
			Event:      "worker.status",
			Data:       status,
			Serializer: &okapi.JSONSerializer{},
		}
		if _, err := msg.Send(w); err != nil {
			return err
		}
		return nil
	}

	// Send initial status immediately.
	if err := sendStatus(); err != nil {
		return nil
	}

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := sendStatus(); err != nil {
				return nil
			}
		}
	}
}

func (h *AdminHandler) buildWorkerStatus() WorkerStatus {
	var status WorkerStatus
	if h.inspector == nil {
		return status
	}
	servers, err := h.inspector.Servers()
	if err != nil {
		return status
	}
	selfPID := os.Getpid()
	selfHost, _ := os.Hostname()
	status.ActiveWorkers = len(servers)
	status.Workers = make([]WorkerDetail, 0, len(servers))
	for _, s := range servers {
		wType := "standalone"
		if h.embeddedWorker && s.PID == selfPID && s.Host == selfHost {
			wType = "embedded"
		}
		status.Workers = append(status.Workers, WorkerDetail{
			Host:   s.Host,
			PID:    s.PID,
			Queues: s.Queues,
			Type:   wType,
		})
	}
	return status
}
