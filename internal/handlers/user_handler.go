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
	"net/http"
	"time"

	"strings"

	"github.com/google/uuid"
	"github.com/goposta/posta/internal/dto"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/emailverify"
	"github.com/goposta/posta/internal/services/eventbus"
	"github.com/goposta/posta/internal/services/notification"
	"github.com/goposta/posta/internal/services/seeder"
	"github.com/goposta/posta/internal/services/settings"
	"github.com/goposta/posta/internal/services/twofactor"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/logger"
	"github.com/jkaninda/okapi"
	"golang.org/x/crypto/bcrypt"
)

type UserHandler struct {
	repo          *repositories.UserRepository
	sessionRepo   *repositories.SessionRepository
	jwtSecret     []byte
	seeder        *seeder.Seeder
	bus           *eventbus.EventBus
	settings      *settings.Provider
	notifier      *notification.Service
	emailVerifier *emailverify.Service
}

func NewUserHandler(repo *repositories.UserRepository, jwtSecret string, seeder *seeder.Seeder, bus *eventbus.EventBus) *UserHandler {
	return &UserHandler{
		repo:      repo,
		jwtSecret: []byte(jwtSecret),
		seeder:    seeder,
		bus:       bus,
	}
}

func (h *UserHandler) SetSessionRepo(repo *repositories.SessionRepository) {
	h.sessionRepo = repo
}

const jwtTokenTTL = 24 * time.Hour

func (h *UserHandler) SetSettings(s *settings.Provider) {
	h.settings = s
}

// SetNotifier sets the notification service for sending welcome and password change emails.
func (h *UserHandler) SetNotifier(n *notification.Service) {
	h.notifier = n
}

// SetEmailVerifier wires the email verification service.
func (h *UserHandler) SetEmailVerifier(s *emailverify.Service) {
	h.emailVerifier = s
}

type LoginRequest struct {
	Body struct {
		Email         string `json:"email" required:"true" format:"email"`
		Password      string `json:"password" required:"true" minLength:"4"`
		TwoFactorCode string `json:"two_factor_code"`
	} `json:"body"`
}

type AuthResponse struct {
	Token string `json:"token"`
	User  struct {
		ID    uint            `json:"id"`
		Name  string          `json:"name"`
		Email string          `json:"email"`
		Role  models.UserRole `json:"role"`
	} `json:"user"`
}

type UserProfile struct {
	ID                        uint            `json:"id"`
	Name                      string          `json:"name"`
	Email                     string          `json:"email"`
	Role                      models.UserRole `json:"role"`
	TwoFactorEnabled          bool            `json:"two_factor_enabled"`
	RequireVerifiedDomain     bool            `json:"require_verified_domain"`
	ScheduledDeletionAt       *time.Time      `json:"scheduled_deletion_at"`
	EmailVerifiedAt           *time.Time      `json:"email_verified_at"`
	EmailVerificationRequired bool            `json:"email_verification_required"`
	CreatedAt                 time.Time       `json:"created_at"`
}

type Enable2FAResponse struct {
	Secret string `json:"secret"`
	URL    string `json:"url"` // otpauth:// URL for QR code
}

type Verify2FARequest struct {
	Body struct {
		Code string `json:"code" required:"true" minLength:"6" maxLength:"6"`
	} `json:"body"`
}

type Disable2FARequest struct {
	Body struct {
		Code string `json:"code" required:"true" minLength:"6" maxLength:"6"`
	} `json:"body"`
}

type UpdateProfileRequest struct {
	Body struct {
		Name                  string `json:"name" required:"true" minLength:"1"`
		RequireVerifiedDomain *bool  `json:"require_verified_domain"`
	} `json:"body"`
}

type ChangePasswordRequest struct {
	Body struct {
		CurrentPassword string `json:"current_password" required:"true"`
		NewPassword     string `json:"new_password" required:"true" minLength:"8"`
	} `json:"body"`
}

type RegisterRequest struct {
	Body struct {
		Name     string `json:"name" required:"true" minLength:"1"`
		Email    string `json:"email" required:"true" format:"email"`
		Password string `json:"password" required:"true" minLength:"8"`
	} `json:"body"`
}

// Register allows new users to self-register when registration is enabled.
func (h *UserHandler) Register(c *okapi.Context, req *RegisterRequest) error {
	if h.settings == nil || !h.settings.RegistrationEnabled() {
		return c.AbortForbidden("registration is disabled")
	}

	email := strings.TrimSpace(strings.ToLower(req.Body.Email))
	if email == "" {
		return c.AbortBadRequest("email is required")
	}

	// Check allowed signup domains
	allowedDomains := h.settings.GetString("allowed_signup_domains", "")
	if allowedDomains != "" {
		parts := strings.SplitN(email, "@", 2)
		if len(parts) != 2 {
			return c.AbortBadRequest("invalid email address")
		}
		domain := parts[1]
		allowed := false
		for _, d := range strings.Split(allowedDomains, ",") {
			if strings.TrimSpace(d) == domain {
				allowed = true
				break
			}
		}
		if !allowed {
			return c.AbortForbidden("registration is not allowed for this email domain")
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Body.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.AbortInternalServerError("failed to hash password", err)
	}

	user := &models.User{
		Name:         strings.TrimSpace(req.Body.Name),
		Email:        email,
		PasswordHash: string(hash),
		Role:         models.UserRoleUser,
	}

	if err := h.repo.Create(user); err != nil {
		return c.AbortConflict("email already registered")
	}

	// Seed default data for the new user
	if h.seeder != nil {
		go h.seeder.SeedUserDefaults(user.ID, user.Name)
	}

	if h.bus != nil {
		h.bus.PublishSimple(models.EventCategoryUser, "user.registered", &user.ID, user.Email, c.RealIP(),
			fmt.Sprintf("User %q registered", user.Email), nil)
	}

	// Email verification: if the notifier is configured, send a verification email.
	// Otherwise (self-hosted without SMTP) auto-verify so the account isn't locked out.
	// The welcome email is deferred until after verification (see VerifyEmail).
	if h.emailVerifier != nil {
		if h.emailVerifier.Required() {
			go func() {
				if err := h.emailVerifier.IssueAndSend(user); err != nil {
					logger.Error("failed to send verification email", "user_id", user.ID, "err", err)
				}
			}()
		} else {
			_ = h.emailVerifier.MarkVerifiedNow(user)
		}
	}

	// Auto-login: generate JWT token
	token, _, err := h.generateTokenWithSession(c, user)
	if err != nil {
		return c.AbortInternalServerError("failed to generate token", err)
	}

	return created(c, AuthResponse{
		Token: token,
		User: struct {
			ID    uint            `json:"id"`
			Name  string          `json:"name"`
			Email string          `json:"email"`
			Role  models.UserRole `json:"role"`
		}{ID: user.ID, Name: user.Name, Email: user.Email, Role: user.Role},
	})
}

// RegistrationStatus returns whether registration is enabled (public endpoint).
func (h *UserHandler) RegistrationStatus(c *okapi.Context) error {
	enabled := h.settings != nil && h.settings.RegistrationEnabled()
	return ok(c, okapi.M{"registration_enabled": enabled})
}

type VerifyEmailRequest struct {
	Token string `query:"token" required:"true"`
}

// VerifyEmail redeems a verification token and marks the user's email as verified.
func (h *UserHandler) VerifyEmail(c *okapi.Context, req *VerifyEmailRequest) error {
	if h.emailVerifier == nil {
		return c.AbortBadRequest("email verification is not enabled")
	}
	user, newlyVerified, err := h.emailVerifier.Redeem(req.Token)
	if err != nil {
		return c.AbortBadRequest(err.Error())
	}
	if newlyVerified {
		if h.bus != nil {
			h.bus.PublishSimple(models.EventCategoryUser, "user.email_verified", &user.ID, user.Email, c.RealIP(),
				fmt.Sprintf("User %q verified email", user.Email), nil)
		}
		// Fire the welcome email now that we know the address works.
		if h.notifier != nil {
			go func(uid uint) {
				_ = h.notifier.SendToUser(uid, "Welcome to Posta!", notification.TemplateWelcome, nil)
			}(user.ID)
		}
	}
	return ok(c, okapi.M{"message": "email verified"})
}

// ResendVerificationEmail issues a fresh token for the authenticated user.
func (h *UserHandler) ResendVerificationEmail(c *okapi.Context) error {
	if h.emailVerifier == nil || !h.emailVerifier.Required() {
		return c.AbortBadRequest("email verification is not enabled")
	}
	userID := c.GetInt("user_id")
	user, err := h.repo.FindByID(uint(userID))
	if err != nil {
		return c.AbortNotFound("user not found")
	}
	if user.EmailVerifiedAt != nil {
		return ok(c, okapi.M{"message": "email already verified"})
	}
	if err := h.emailVerifier.CanResend(user.ID); err != nil {
		return c.AbortTooManyRequests(err.Error())
	}
	if err := h.emailVerifier.IssueAndSend(user); err != nil {
		logger.Error("failed to resend verification email", "user_id", user.ID, "err", err)
		return c.AbortInternalServerError("failed to send verification email")
	}
	return ok(c, okapi.M{"message": "verification email sent"})
}

// UpdateProfile allows authenticated users to update their profile (name).
func (h *UserHandler) UpdateProfile(c *okapi.Context, req *UpdateProfileRequest) error {
	userID := c.GetInt("user_id")
	user, err := h.repo.FindByID(uint(userID))
	if err != nil {
		return c.AbortNotFound("user not found")
	}

	user.Name = req.Body.Name
	if req.Body.RequireVerifiedDomain != nil {
		user.RequireVerifiedDomain = *req.Body.RequireVerifiedDomain
	}
	if err := h.repo.Update(user); err != nil {
		return c.AbortInternalServerError("failed to update profile")
	}

	return ok(c, h.buildProfile(user))
}

// ChangePassword allows authenticated users to change their own password.
func (h *UserHandler) ChangePassword(c *okapi.Context, req *ChangePasswordRequest) error {
	userID := c.GetInt("user_id")
	user, err := h.repo.FindByID(uint(userID))
	if err != nil {
		return c.AbortNotFound("user not found")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Body.CurrentPassword)); err != nil {
		return c.AbortBadRequest("current password is incorrect")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Body.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return c.AbortInternalServerError("failed to hash password", err)
	}

	user.PasswordHash = string(hash)
	if err := h.repo.Update(user); err != nil {
		return c.AbortInternalServerError("failed to update password")
	}

	// Send password change notification (best-effort)
	if h.notifier != nil {
		go func() {
			_ = h.notifier.SendToUser(user.ID, "Your password has been changed", notification.TemplatePasswordChanged, map[string]any{
				"ChangedAt": time.Now().UTC().Format("January 2, 2006 at 15:04 UTC"),
			})
		}()
	}

	return ok(c, okapi.M{"message": "password updated successfully"})
}

func (h *UserHandler) publishLoginFailed(actorID *uint, email, ip, reason string) {
	if h.bus == nil {
		return
	}
	h.bus.PublishSimple(models.EventCategoryUser, "user.login_failed", actorID, email, ip,
		fmt.Sprintf("Failed login attempt for %q (%s)", email, reason),
		map[string]any{"reason": reason})
}

func (h *UserHandler) Login(c *okapi.Context, req *LoginRequest) error {
	user, err := h.repo.FindByEmail(req.Body.Email)
	if err != nil {
		h.publishLoginFailed(nil, req.Body.Email, c.RealIP(), "unknown_email")
		return c.AbortUnauthorized("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Body.Password)); err != nil {
		h.publishLoginFailed(&user.ID, user.Email, c.RealIP(), "bad_password")
		return c.AbortUnauthorized("invalid credentials")
	}

	if !user.Active {
		h.publishLoginFailed(&user.ID, user.Email, c.RealIP(), "inactive")
		return c.AbortForbidden("account is disabled")
	}

	// Check 2FA
	if user.TwoFactorEnabled {
		if req.Body.TwoFactorCode == "" {
			return c.JSON(http.StatusUnauthorized, dto.Response[any]{
				Success: false,
				Data: okapi.M{
					"requires_2fa": true,
					"message":      "2FA code required",
				},
			})
		}
		if !twofactor.ValidateCode(user.TwoFactorSecret, req.Body.TwoFactorCode) {
			h.publishLoginFailed(&user.ID, user.Email, c.RealIP(), "bad_2fa")
			return c.AbortUnauthorized("invalid 2FA code")
		}
	}

	// Seed default data on first login
	go h.seeder.SeedUserDefaults(user.ID, user.Name)

	// Record last login time
	now := time.Now()
	user.LastLoginAt = &now
	_ = h.repo.Update(user)

	if h.bus != nil {
		h.bus.PublishSimple(models.EventCategoryUser, "user.login", &user.ID, user.Email, c.RealIP(),
			fmt.Sprintf("User %q logged in", user.Email), nil)
	}

	token, jti, err := h.generateTokenWithSession(c, user)
	if err != nil {
		return c.AbortInternalServerError("failed to generate token", err)
	}
	_ = jti

	return ok(c, AuthResponse{
		Token: token,
		User: struct {
			ID    uint            `json:"id"`
			Name  string          `json:"name"`
			Email string          `json:"email"`
			Role  models.UserRole `json:"role"`
		}{ID: user.ID, Name: user.Name, Email: user.Email, Role: user.Role},
	})
}

// Me returns the current user's profile.
func (h *UserHandler) Me(c *okapi.Context) error {
	userID := c.GetInt("user_id")
	user, err := h.repo.FindByID(uint(userID))
	if err != nil {
		return c.AbortNotFound("user not found")
	}

	return ok(c, h.buildProfile(user))
}

func (h *UserHandler) buildProfile(user *models.User) UserProfile {
	required := h.emailVerifier != nil && h.emailVerifier.Required()
	return UserProfile{
		ID:                        user.ID,
		Name:                      user.Name,
		Email:                     user.Email,
		Role:                      user.Role,
		TwoFactorEnabled:          user.TwoFactorEnabled,
		RequireVerifiedDomain:     user.RequireVerifiedDomain,
		ScheduledDeletionAt:       user.ScheduledDeletionAt,
		EmailVerifiedAt:           user.EmailVerifiedAt,
		EmailVerificationRequired: required,
		CreatedAt:                 user.CreatedAt,
	}
}

// Setup2FA generates a TOTP secret for the user (doesn't enable yet).
func (h *UserHandler) Setup2FA(c *okapi.Context) error {
	userID := c.GetInt("user_id")
	user, err := h.repo.FindByID(uint(userID))
	if err != nil {
		return c.AbortNotFound("user not found")
	}
	if user.TwoFactorEnabled {
		return c.AbortBadRequest("2FA is already enabled")
	}

	secret, url, err := twofactor.GenerateSecret(user.Email)
	if err != nil {
		return c.AbortInternalServerError("failed to generate 2FA secret")
	}

	// Store secret temporarily (not enabled yet)
	user.TwoFactorSecret = secret
	if err := h.repo.Update(user); err != nil {
		return c.AbortInternalServerError("failed to save 2FA secret")
	}

	return ok(c, Enable2FAResponse{
		Secret: secret,
		URL:    url,
	})
}

// Verify2FA verifies a TOTP code and enables 2FA.
func (h *UserHandler) Verify2FA(c *okapi.Context, req *Verify2FARequest) error {
	userID := c.GetInt("user_id")
	user, err := h.repo.FindByID(uint(userID))
	if err != nil {
		return c.AbortNotFound("user not found")
	}
	if user.TwoFactorEnabled {
		return c.AbortBadRequest("2FA is already enabled")
	}
	if user.TwoFactorSecret == "" {
		return c.AbortBadRequest("2FA setup not initiated, call setup first")
	}

	if !twofactor.ValidateCode(user.TwoFactorSecret, req.Body.Code) {
		return c.AbortBadRequest("invalid 2FA code")
	}

	user.TwoFactorEnabled = true
	if err := h.repo.Update(user); err != nil {
		return c.AbortInternalServerError("failed to enable 2FA")
	}

	return ok(c, okapi.M{"message": "2FA enabled successfully"})
}

// Disable2FA disables 2FA after verifying a code.
func (h *UserHandler) Disable2FA(c *okapi.Context, req *Disable2FARequest) error {
	userID := c.GetInt("user_id")
	user, err := h.repo.FindByID(uint(userID))
	if err != nil {
		return c.AbortNotFound("user not found")
	}
	if !user.TwoFactorEnabled {
		return c.AbortBadRequest("2FA is not enabled")
	}

	if !twofactor.ValidateCode(user.TwoFactorSecret, req.Body.Code) {
		return c.AbortBadRequest("invalid 2FA code")
	}

	user.TwoFactorEnabled = false
	user.TwoFactorSecret = ""
	if err := h.repo.Update(user); err != nil {
		return c.AbortInternalServerError("failed to disable 2FA")
	}

	return ok(c, okapi.M{"message": "2FA disabled successfully"})
}

// generateTokenWithSession creates a JWT with a jti claim and records the session.
func (h *UserHandler) generateTokenWithSession(c *okapi.Context, user *models.User) (string, string, error) { //nolint:unparam
	jti := uuid.NewString()

	token, err := okapi.GenerateJwtToken(h.jwtSecret, map[string]any{
		"sub":   user.ID,
		"email": user.Email,
		"role":  string(user.Role),
		"aud":   "posta",
		"jti":   jti,
	}, jwtTokenTTL)
	if err != nil {
		return "", "", err
	}

	// Track session in database
	if h.sessionRepo != nil {
		ua := c.Header("User-Agent")
		if len(ua) > 512 {
			ua = ua[:512]
		}
		session := &models.Session{
			UserID:    user.ID,
			JTI:       jti,
			IPAddress: c.RealIP(),
			UserAgent: ua,
			ExpiresAt: time.Now().Add(jwtTokenTTL),
		}
		_ = h.sessionRepo.Create(session)
	}

	return token, jti, nil
}

// RequestAccountDeletion schedules the current user's account for deletion in 7 days.
func (h *UserHandler) RequestAccountDeletion(c *okapi.Context) error {
	userID := c.GetInt("user_id")
	user, err := h.repo.FindByID(uint(userID))
	if err != nil {
		return c.AbortNotFound("user not found")
	}

	if user.Role == models.UserRoleAdmin {
		return c.AbortBadRequest("admin accounts cannot be self-deleted")
	}

	if user.ScheduledDeletionAt != nil {
		return c.AbortBadRequest("account deletion is already scheduled")
	}

	deletionDate := time.Now().Add(7 * 24 * time.Hour)
	user.ScheduledDeletionAt = &deletionDate
	user.Active = false

	if err := h.repo.Update(user); err != nil {
		return c.AbortInternalServerError("failed to schedule account deletion")
	}

	if h.bus != nil {
		uid := user.ID
		h.bus.PublishSimple(models.EventCategoryUser, "user.deletion_requested", &uid, user.Email, c.RealIP(),
			fmt.Sprintf("Account deletion scheduled for %s", deletionDate.Format("2006-01-02")), nil)
	}

	return ok(c, map[string]any{
		"message":               "Account scheduled for deletion",
		"scheduled_deletion_at": deletionDate,
	})
}

// CancelAccountDeletion cancels a previously scheduled account deletion.
func (h *UserHandler) CancelAccountDeletion(c *okapi.Context) error {
	userID := c.GetInt("user_id")
	user, err := h.repo.FindByID(uint(userID))
	if err != nil {
		return c.AbortNotFound("user not found")
	}

	if user.ScheduledDeletionAt == nil {
		return c.AbortBadRequest("no deletion is scheduled")
	}

	user.ScheduledDeletionAt = nil
	user.Active = true

	if err := h.repo.Update(user); err != nil {
		return c.AbortInternalServerError("failed to cancel account deletion")
	}

	if h.bus != nil {
		uid := user.ID
		h.bus.PublishSimple(models.EventCategoryUser, "user.deletion_cancelled", &uid, user.Email, c.RealIP(),
			"Account deletion cancelled", nil)
	}

	return ok(c, map[string]any{"message": "Account deletion cancelled"})
}
