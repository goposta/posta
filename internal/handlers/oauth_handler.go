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

package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/auth"
	"github.com/goposta/posta/internal/services/eventbus"
	"github.com/goposta/posta/internal/services/seeder"
	sessionpkg "github.com/goposta/posta/internal/services/session"
	"github.com/goposta/posta/internal/services/workspaceprovision"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/logger"
	"github.com/jkaninda/okapi"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type OAuthHandler struct {
	oauthService *auth.OAuthService
	providerRepo *repositories.OAuthProviderRepository
	accountRepo  *repositories.OAuthAccountRepository
	userRepo     *repositories.UserRepository
	sessionRepo  *repositories.SessionRepository
	jwtSecret    []byte
	bus          *eventbus.EventBus
	seeder       *seeder.Seeder
	redisClient  *redis.Client
	callbackBase string // e.g. "http://localhost:9000"
	appWebURL    string // e.g. "http://localhost:9000"
	db           *gorm.DB
	migrator     *workspaceprovision.Service
}

func (h *OAuthHandler) SetMigrator(db *gorm.DB, m *workspaceprovision.Service) {
	h.db = db
	h.migrator = m
}

func NewOAuthHandler(
	oauthService *auth.OAuthService,
	providerRepo *repositories.OAuthProviderRepository,
	accountRepo *repositories.OAuthAccountRepository,
	userRepo *repositories.UserRepository,
	sessionRepo *repositories.SessionRepository,
	jwtSecret string,
	seeder *seeder.Seeder,
	bus *eventbus.EventBus,
	redisClient *redis.Client,
	callbackBase string,
	appWebURL string,
) *OAuthHandler {
	return &OAuthHandler{
		oauthService: oauthService,
		providerRepo: providerRepo,
		accountRepo:  accountRepo,
		userRepo:     userRepo,
		sessionRepo:  sessionRepo,
		jwtSecret:    []byte(jwtSecret),
		seeder:       seeder,
		bus:          bus,
		redisClient:  redisClient,
		callbackBase: callbackBase,
		appWebURL:    appWebURL,
	}
}

type OAuthProviderInfo struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Type        string `json:"type"`
}

type OAuthLinkedAccount struct {
	ID           uint   `json:"id"`
	ProviderID   uint   `json:"provider_id"`
	ProviderName string `json:"provider_name"`
	ProviderType string `json:"provider_type"`
	Email        string `json:"email"`
	CreatedAt    string `json:"created_at"`
}

type UnlinkOAuthRequest struct {
	ProviderID int `param:"provider_id"`
}

type DiscoverSSORequest struct {
	Body struct {
		Email string `json:"email" required:"true"`
	} `json:"body"`
}

type DiscoverSSOResponse struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Type        string `json:"type"`
}

func (h *OAuthHandler) ListProviders(c *okapi.Context) error {
	providers, err := h.providerRepo.FindEnabled()
	if err != nil {
		return ok(c, okapi.M{"providers": []OAuthProviderInfo{}, "sso_available": false})
	}

	var result []OAuthProviderInfo
	for _, p := range providers {
		result = append(result, OAuthProviderInfo{
			ID:          p.ID,
			Name:        p.Name,
			DisplayName: p.DisplayName,
			Type:        string(p.Type),
		})
	}

	ssoAvailable, _ := h.providerRepo.HasEnabledHidden()

	return ok(c, okapi.M{"providers": result, "sso_available": ssoAvailable})
}

// DiscoverSSO finds an enabled provider whose AllowedDomains match the email's domain.
func (h *OAuthHandler) DiscoverSSO(c *okapi.Context, req *DiscoverSSORequest) error {
	email := strings.TrimSpace(req.Body.Email)
	at := strings.LastIndex(email, "@")
	if at < 0 || at == len(email)-1 {
		return c.AbortBadRequest("invalid email")
	}
	domain := email[at+1:]

	provider, err := h.providerRepo.FindEnabledByDomain(domain)
	if err != nil {
		return c.AbortNotFound("no SSO provider for this email")
	}

	return ok(c, DiscoverSSOResponse{
		Name:        provider.Name,
		DisplayName: provider.DisplayName,
		Type:        string(provider.Type),
	})
}

func (h *OAuthHandler) Authorize(c *okapi.Context) error {
	providerName := c.Param("provider")
	provider, err := h.providerRepo.FindByName(providerName)
	if err != nil {
		return c.AbortNotFound("OAuth provider not found")
	}

	// Generate state for CSRF protection
	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		return c.AbortInternalServerError("failed to generate state")
	}
	state := hex.EncodeToString(stateBytes)

	stateData, _ := json.Marshal(map[string]string{
		"provider_slug": provider.Name,
		"created_at":    time.Now().UTC().Format(time.RFC3339),
	})
	h.redisClient.Set(c.Request().Context(), "oauth:state:"+state, string(stateData), 10*time.Minute)

	redirectURI := fmt.Sprintf("%s/api/v1/auth/oauth/%s/callback", h.callbackBase, provider.Name)
	oauthCfg, err := h.oauthService.GetOAuthConfig(provider, redirectURI)
	if err != nil {
		return c.AbortBadRequest("OAuth provider misconfigured: " + err.Error())
	}
	authURL := oauthCfg.AuthCodeURL(state)

	c.Redirect(http.StatusFound, authURL)
	return nil
}

func (h *OAuthHandler) Callback(c *okapi.Context) error {
	providerName := c.Param("provider")
	code := c.Query("code")
	state := c.Query("state")
	oauthError := c.Query("error")

	redirect := func(errMsg string) error {
		c.Redirect(http.StatusFound, h.appWebURL+"/login?error="+errMsg)
		return nil
	}

	if oauthError != "" {
		return redirect(oauthError)
	}
	if code == "" || state == "" {
		return redirect("missing_params")
	}

	ctx := c.Request().Context()
	stateKey := "oauth:state:" + state
	stateVal := h.redisClient.Get(ctx, stateKey)
	if stateVal.Err() != nil {
		return redirect("invalid_state")
	}
	h.redisClient.Del(ctx, stateKey)

	var stateData map[string]string
	if err := json.Unmarshal([]byte(stateVal.Val()), &stateData); err != nil || stateData["provider_slug"] != providerName {
		return redirect("state_mismatch")
	}

	provider, err := h.providerRepo.FindByName(providerName)
	if err != nil {
		return redirect("provider_not_found")
	}

	redirectURI := fmt.Sprintf("%s/api/v1/auth/oauth/%s/callback", h.callbackBase, provider.Name)

	userInfo, token, err := h.oauthService.ExchangeCode(ctx, provider, code, redirectURI)
	if err != nil {
		logger.Error("OAuth exchange failed for provider %s: %v", provider.Name, err)
		return redirect("exchange_failed")
	}

	user, isNew, err := h.oauthService.FindOrCreateUser(provider, userInfo, token)
	if err != nil {
		logger.Error("OAuth account resolution failed for provider %s: %v", provider.Name, err)
		return redirect("account_error")
	}

	if !user.Active {
		return redirect("account_disabled")
	}

	// Provisioned before issuing the JWT; idempotent for already-migrated returning users.
	if h.migrator != nil && h.db != nil {
		if _, err := h.migrator.EnsureWorkspace(h.db, user.ID); err != nil {
			logger.Error("failed to provision personal workspace", "user_id", user.ID, "err", err)
		}
	}

	jwtToken, err := h.generateToken(c, user)
	if err != nil {
		return redirect("token_generation_failed")
	}

	if h.bus != nil {
		action := "user.oauth_login"
		if isNew {
			action = "user.oauth_registered"
		}
		h.bus.PublishSimple(models.EventCategoryUser, action, &user.ID, user.Email, c.RealIP(),
			fmt.Sprintf("User %q authenticated via %s", user.Email, provider.DisplayName), nil)
	}

	now := time.Now()
	user.LastLoginAt = &now
	_ = h.userRepo.Update(user)

	// A cookie, not a query parameter, so the JWT never lands in browser history or a Referer header.
	sessionpkg.SetCookie(c, jwtToken, jwtTokenTTL)
	c.Redirect(http.StatusFound, h.appWebURL+"/auth/oauth/callback")
	return nil
}

func (h *OAuthHandler) ListLinkedAccounts(c *okapi.Context) error {
	userID := c.GetInt("user_id")

	accounts, err := h.accountRepo.FindByUserID(uint(userID))
	if err != nil {
		return c.AbortInternalServerError("failed to list linked accounts")
	}

	var result []OAuthLinkedAccount
	for _, a := range accounts {
		result = append(result, OAuthLinkedAccount{
			ID:           a.ID,
			ProviderID:   a.ProviderID,
			ProviderName: a.Provider.DisplayName,
			ProviderType: string(a.Provider.Type),
			Email:        a.Email,
			CreatedAt:    a.CreatedAt.Format(time.RFC3339),
		})
	}

	return ok(c, result)
}

func (h *OAuthHandler) UnlinkAccount(c *okapi.Context, req *UnlinkOAuthRequest) error {
	userID := c.GetInt("user_id")

	// Check the user has another auth method before unlinking
	user, err := h.userRepo.FindByID(uint(userID))
	if err != nil {
		return c.AbortNotFound("user not found")
	}

	account, err := h.accountRepo.FindByUserAndProvider(uint(userID), uint(req.ProviderID))
	if err != nil {
		return c.AbortNotFound("linked account not found")
	}

	count, _ := h.accountRepo.CountByUserID(uint(userID))
	hasPassword := user.PasswordHash != "" && user.AuthMethod != "oauth"

	if count <= 1 && !hasPassword {
		return c.AbortBadRequest("cannot unlink the last authentication method — set a password first")
	}

	if err := h.accountRepo.Delete(account.ID); err != nil {
		return c.AbortInternalServerError("failed to unlink account")
	}

	if count <= 1 && hasPassword {
		user.AuthMethod = "password"
		_ = h.userRepo.Update(user)
	}

	return ok(c, okapi.M{"message": "account unlinked"})
}

func (h *OAuthHandler) generateToken(c *okapi.Context, user *models.User) (string, error) {
	jti := uuid.NewString()

	token, err := okapi.GenerateJwtToken(h.jwtSecret, map[string]any{
		"sub":   user.ID,
		"email": user.Email,
		"role":  string(user.Role),
		"aud":   "posta",
		"jti":   jti,
	}, jwtTokenTTL)
	if err != nil {
		return "", err
	}

	if h.sessionRepo != nil {
		session := &models.Session{
			UserID:    user.ID,
			JTI:       jti,
			IPAddress: c.RealIP(),
			UserAgent: c.Request().UserAgent(),
			ExpiresAt: time.Now().Add(jwtTokenTTL),
		}
		_ = h.sessionRepo.Create(session)
	}

	return token, nil
}
