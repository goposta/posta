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

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/cache"
	"github.com/goposta/posta/internal/services/eventbus"
	"github.com/goposta/posta/internal/services/seeder"
	"github.com/goposta/posta/internal/services/session"
	"github.com/goposta/posta/internal/services/settings"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/hibiken/asynq"
	"github.com/jkaninda/okapi"
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
	workspaceRepo  *repositories.WorkspaceRepository
	planRepo       *repositories.PlanRepository
	inspector      *asynq.Inspector
	bus            *eventbus.EventBus
	seeder         *seeder.Seeder
	embeddedWorker bool
	emailSettings  *settings.Provider
	sessionRepo    *repositories.SessionRepository
	sessionStore   *session.Store
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

// SetWorkspaceRepo sets the workspace and plan repositories for workspace management.
func (h *AdminHandler) SetWorkspaceRepo(wsRepo *repositories.WorkspaceRepository, planRepo *repositories.PlanRepository) {
	h.workspaceRepo = wsRepo
	h.planRepo = planRepo
}

func (h *AdminHandler) SetEmailSettings(s *settings.Provider) {
	h.emailSettings = s
}

// SetSessionRepo sets the session repository and store for session management.
func (h *AdminHandler) SetSessionRepo(repo *repositories.SessionRepository, store *session.Store) {
	h.sessionRepo = repo
	h.sessionStore = store
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

// DeleteUser schedules a user for deletion (admin only).
func (h *AdminHandler) DeleteUser(c *okapi.Context, req *AdminDeleteUserRequest) error {
	currentUserID := c.GetInt("user_id")
	if req.ID == currentUserID {
		return c.AbortBadRequest("cannot delete your own account")
	}

	user, err := h.userRepo.FindByID(uint(req.ID))
	if err != nil {
		return c.AbortNotFound("user not found")
	}

	if user.ScheduledDeletionAt != nil {
		return c.AbortBadRequest("account deletion is already scheduled")
	}

	deletionDate := time.Now().Add(7 * 24 * time.Hour)
	user.ScheduledDeletionAt = &deletionDate
	user.Active = false

	if err := h.userRepo.Update(user); err != nil {
		return c.AbortInternalServerError("failed to schedule account deletion")
	}

	if h.bus != nil {
		adminID := uint(c.GetInt("user_id"))
		h.bus.PublishSimple(models.EventCategoryUser, "user.deletion_scheduled", &adminID, c.GetString("email"), c.RealIP(),
			fmt.Sprintf("User ID %d scheduled for deletion on %s", req.ID, deletionDate.Format("2006-01-02")),
			map[string]any{"deleted_user_id": req.ID, "scheduled_deletion_at": deletionDate})
	}

	return ok(c, map[string]any{
		"message":               "Account disabled and scheduled for deletion",
		"scheduled_deletion_at": deletionDate,
	})
}

// CancelUserDeletion cancels a scheduled account deletion (admin only).
func (h *AdminHandler) CancelUserDeletion(c *okapi.Context, req *AdminDeleteUserRequest) error {
	user, err := h.userRepo.FindByID(uint(req.ID))
	if err != nil {
		return c.AbortNotFound("user not found")
	}

	if user.ScheduledDeletionAt == nil {
		return c.AbortBadRequest("no deletion is scheduled for this account")
	}

	user.ScheduledDeletionAt = nil
	user.Active = true

	if err := h.userRepo.Update(user); err != nil {
		return c.AbortInternalServerError("failed to cancel account deletion")
	}

	if h.bus != nil {
		adminID := uint(c.GetInt("user_id"))
		h.bus.PublishSimple(models.EventCategoryUser, "user.deletion_cancelled", &adminID, c.GetString("email"), c.RealIP(),
			fmt.Sprintf("Scheduled deletion cancelled for user ID %d", req.ID),
			map[string]any{"user_id": req.ID})
	}

	return ok(c, map[string]any{"message": "Account deletion cancelled"})
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

// AdminWorkspace is a workspace with its plan name for admin views.
type AdminWorkspace struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	OwnerID   uint      `json:"owner_id"`
	PlanID    *uint     `json:"plan_id"`
	PlanName  string    `json:"plan_name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UserWorkspaces returns workspaces for a specific user (admin only).
func (h *AdminHandler) UserWorkspaces(c *okapi.Context, req *AdminGetUserRequest) error {
	_, err := h.userRepo.FindByID(uint(req.ID))
	if err != nil {
		return c.AbortNotFound("user not found")
	}

	workspaces, err := h.workspaceRepo.FindByUserID(uint(req.ID))
	if err != nil {
		return c.AbortInternalServerError("failed to list workspaces")
	}

	// Collect plan IDs and fetch plan names
	planIDs := make(map[uint]bool)
	for _, ws := range workspaces {
		if ws.PlanID != nil {
			planIDs[*ws.PlanID] = true
		}
	}

	planNames := make(map[uint]string)
	if h.planRepo != nil {
		for id := range planIDs {
			if plan, err := h.planRepo.FindByID(id); err == nil {
				planNames[plan.ID] = plan.Name
			}
		}
	}

	result := make([]AdminWorkspace, len(workspaces))
	for i, ws := range workspaces {
		result[i] = AdminWorkspace{
			ID:        ws.ID,
			Name:      ws.Name,
			Slug:      ws.Slug,
			OwnerID:   ws.OwnerID,
			PlanID:    ws.PlanID,
			CreatedAt: ws.CreatedAt,
			UpdatedAt: ws.UpdatedAt,
		}
		if ws.PlanID != nil {
			result[i].PlanName = planNames[*ws.PlanID]
		}
	}

	return ok(c, result)
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

// RevokeUserSessions revokes all active sessions for a user.
func (h *AdminHandler) RevokeUserSessions(c *okapi.Context, req *AdminGetUserRequest) error {
	user, err := h.userRepo.FindByID(uint(req.ID))
	if err != nil {
		return c.AbortNotFound("user not found")
	}

	// Get active sessions first
	sessions, err := h.sessionRepo.FindActiveByUserID(user.ID)
	if err != nil {
		return c.AbortInternalServerError("failed to load sessions")
	}

	count, err := h.sessionRepo.RevokeAllByUserID(user.ID)
	if err != nil {
		return c.AbortInternalServerError("failed to revoke sessions")
	}

	for _, s := range sessions {
		h.sessionStore.MarkRevoked(c.Request().Context(), s.JTI, s.ExpiresAt)
	}

	if h.bus != nil {
		adminID := uint(c.GetInt("user_id"))
		h.bus.PublishSimple(models.EventCategoryUser, "user.sessions_revoked", &adminID, c.GetString("email"), c.RealIP(),
			fmt.Sprintf("All sessions revoked for user %q by admin", user.Email), map[string]any{"user_id": user.ID, "revoked": count})
	}

	return ok(c, okapi.M{"message": fmt.Sprintf("%d session(s) revoked", count), "revoked": count})
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
