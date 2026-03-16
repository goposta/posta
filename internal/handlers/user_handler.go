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
	"net/http"
	"time"

	"strings"

	"github.com/jkaninda/okapi"
	"github.com/jkaninda/posta/internal/dto"
	"github.com/jkaninda/posta/internal/models"
	"github.com/jkaninda/posta/internal/services/eventbus"
	"github.com/jkaninda/posta/internal/services/seeder"
	"github.com/jkaninda/posta/internal/services/settings"
	"github.com/jkaninda/posta/internal/services/twofactor"
	"github.com/jkaninda/posta/internal/storage/repositories"
	"golang.org/x/crypto/bcrypt"
)

type UserHandler struct {
	repo      *repositories.UserRepository
	jwtSecret []byte
	seeder    *seeder.Seeder
	bus       *eventbus.EventBus
	settings  *settings.Provider
}

func NewUserHandler(repo *repositories.UserRepository, jwtSecret string, seeder *seeder.Seeder, bus *eventbus.EventBus) *UserHandler {
	return &UserHandler{
		repo:      repo,
		jwtSecret: []byte(jwtSecret),
		seeder:    seeder,
		bus:       bus,
	}
}

func (h *UserHandler) SetSettings(s *settings.Provider) {
	h.settings = s
}

type LoginRequest struct {
	Body struct {
		Email         string `json:"email" required:"true" format:"email"`
		Password      string `json:"password" required:"true"`
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
	ID                    uint            `json:"id"`
	Name                  string          `json:"name"`
	Email                 string          `json:"email"`
	Role                  models.UserRole `json:"role"`
	TwoFactorEnabled      bool            `json:"two_factor_enabled"`
	RequireVerifiedDomain bool            `json:"require_verified_domain"`
	CreatedAt             time.Time       `json:"created_at"`
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

	// Auto-login: generate JWT token
	token, err := okapi.GenerateJwtToken(h.jwtSecret, map[string]any{
		"sub":   user.ID,
		"email": user.Email,
		"role":  string(user.Role),
		"aud":   "posta",
	}, 24*time.Hour)
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

	return ok(c, UserProfile{
		ID:                    user.ID,
		Name:                  user.Name,
		Email:                 user.Email,
		Role:                  user.Role,
		TwoFactorEnabled:      user.TwoFactorEnabled,
		RequireVerifiedDomain: user.RequireVerifiedDomain,
		CreatedAt:             user.CreatedAt,
	})
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

	return ok(c, okapi.M{"message": "password updated successfully"})
}

func (h *UserHandler) Login(c *okapi.Context, req *LoginRequest) error {
	user, err := h.repo.FindByEmail(req.Body.Email)
	if err != nil {
		return c.AbortUnauthorized("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Body.Password)); err != nil {
		return c.AbortUnauthorized("invalid credentials")
	}

	if !user.Active {
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

	token, err := okapi.GenerateJwtToken(h.jwtSecret, map[string]any{
		"sub":   user.ID,
		"email": user.Email,
		"role":  string(user.Role),
		"aud":   "posta",
	}, 24*time.Hour)
	if err != nil {
		return c.AbortInternalServerError("failed to generate token", err)
	}

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

	return ok(c, UserProfile{
		ID:                    user.ID,
		Name:                  user.Name,
		Email:                 user.Email,
		Role:                  user.Role,
		TwoFactorEnabled:      user.TwoFactorEnabled,
		RequireVerifiedDomain: user.RequireVerifiedDomain,
		CreatedAt:             user.CreatedAt,
	})
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
