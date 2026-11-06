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

// Package middlewares authenticates a user JWT (header or session cookie) or a psk_ API key (header, or ?token=).
package middlewares

import (
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/goposta/posta/internal/config"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/auth"
	"github.com/goposta/posta/internal/services/emailverify"
	"github.com/goposta/posta/internal/services/ratelimit"
	sessionpkg "github.com/goposta/posta/internal/services/session"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
)

const (
	CtxUserID    = "user_id"
	CtxUserEmail = "user_email"
	CtxUserRole  = "role"
	CtxJTI       = "jti"

	CtxAuthMethod        = "auth_method"
	CtxWorkspaceID       = "workspace_id"
	CtxWorkspaceRole     = "workspace_role"
	CtxAPIKeyID          = "api_key_id"
	CtxAPIKeyScopes      = "api_key_scopes"
	CtxAPIKeyWorkspaceID = "api_key_workspace_id"
)

const (
	AuthMethodJWT    = "jwt"
	AuthMethodAPIKey = "api_key"
)

const SessionCookieName = sessionpkg.CookieName

// No query:token on purpose: a user JWT in a URL would leak into proxy logs, browser history and Referer headers.
const tokenLookup = "header:Authorization,cookie:" + SessionCookieName

// okapi sends every auth failure through OnUnauthorized, so this marks a 403 to keep it from signing users out.
const ctxRejection = "auth_rejection"

const (
	rejectionRevoked   = "revoked"
	rejectionForbidden = "forbidden"
)

func baseJWTAuth(cfg *config.Config, store *sessionpkg.Store, requireAdmin bool) okapi.JWTAuth {
	a := okapi.JWTAuth{
		SigningSecret: []byte(cfg.JWTSecret),
		Audience:      "posta",
		ContextKey:    "jwt_user",
		TokenLookup:   tokenLookup,
		ForwardClaims: map[string]string{
			CtxUserID:   "sub",
			"email":     "email",
			CtxUserRole: "role",
			CtxJTI:      "jti",
		},

		ValidateClaims: validateClaims(store, requireAdmin),
	}
	a.OnUnauthorized = func(c *okapi.Context) error {
		switch c.GetString(ctxRejection) {
		case rejectionForbidden:
			return c.AbortForbidden("insufficient permissions")
		case rejectionRevoked:
			return c.AbortUnauthorized("session has been revoked")
		default:
			return c.AbortUnauthorized("invalid or expired session")
		}
	}
	return a
}

// Runs only after okapi accepted the signature and expiry, so the caller holds a genuine session.
func validateClaims(store *sessionpkg.Store, requireAdmin bool) func(c *okapi.Context, claims jwt.Claims) error {
	return func(c *okapi.Context, claims jwt.Claims) error {
		mapClaims, ok := claims.(jwt.MapClaims)
		if !ok {
			return errors.New("invalid token: unexpected claims")
		}

		jti, _ := mapClaims["jti"].(string)
		if jti == "" {
			return errors.New("invalid token: missing jti")
		}
		if store != nil && store.IsRevoked(c.Request().Context(), jti) {
			c.Set(ctxRejection, rejectionRevoked)
			return errors.New("session has been revoked")
		}

		if requireAdmin {
			if role, _ := mapClaims["role"].(string); role != string(models.UserRoleAdmin) {
				c.Set(ctxRejection, rejectionForbidden)
				return errors.New("admin role required")
			}
		}
		return nil
	}
}

// Revoked sessions are rejected when sessionStore is provided.
func JWTAuth(cfg *config.Config, sessionStore ...*sessionpkg.Store) okapi.JWTAuth {
	return baseJWTAuth(cfg, firstStore(sessionStore), false)
}

// An API key is never an admin credential: a psk_ token fails JWT parsing here and is rejected.
func JWTAdminAuth(cfg *config.Config, sessionStore ...*sessionpkg.Store) okapi.JWTAuth {
	return baseJWTAuth(cfg, firstStore(sessionStore), true)
}

func firstStore(stores []*sessionpkg.Store) *sessionpkg.Store {
	if len(stores) > 0 {
		return stores[0]
	}
	return nil
}

// API keys may also come from ?token= so EventSource and download links can authenticate without headers.
func Authenticate(jwtAuth okapi.JWTAuth, keyService *auth.APIKeyService, userRepo *repositories.UserRepository, keyRepo *repositories.APIKeyRepository) okapi.Middleware {
	return func(c *okapi.Context) error {
		raw := bearerToken(c)
		if raw == "" {
			raw = strings.TrimSpace(c.Query("token"))
		}
		if strings.HasPrefix(raw, auth.APIKeyPrefix) {
			return authenticateAPIKey(c, keyService, userRepo, keyRepo, raw)
		}
		if raw == "" {
			// A browser sends the JWT only as an HttpOnly cookie, so let okapi read it via TokenLookup.
			if _, err := c.Cookie(SessionCookieName); err != nil {
				return c.AbortUnauthorized("authentication required")
			}
		}
		c.Set(CtxAuthMethod, AuthMethodJWT)
		return jwtAuth.Middleware(c)
	}
}

// APIKeyAuth takes keys only from the header: a browser session is not valid here and no caller needs a key in a URL.
func APIKeyAuth(keyService *auth.APIKeyService, userRepo *repositories.UserRepository, keyRepo *repositories.APIKeyRepository) okapi.Middleware {
	return func(c *okapi.Context) error {
		raw := bearerToken(c)
		if raw == "" {
			return c.AbortUnauthorized("missing Authorization header")
		}
		if !strings.HasPrefix(raw, auth.APIKeyPrefix) {
			return c.AbortUnauthorized("invalid Authorization format, expected: Bearer <API_KEY>")
		}
		return authenticateAPIKey(c, keyService, userRepo, keyRepo, raw)
	}
}

// The Bearer prefix is optional so a bare `Authorization: psk_…` still works.
func bearerToken(c *okapi.Context) string {
	return strings.TrimSpace(strings.TrimPrefix(c.Header("Authorization"), "Bearer "))
}

func authenticateAPIKey(c *okapi.Context, keyService *auth.APIKeyService, userRepo *repositories.UserRepository, keyRepo *repositories.APIKeyRepository, rawKey string) error {
	apiKey, err := keyService.ValidateKey(rawKey)
	if err != nil {
		return c.AbortUnauthorized("invalid API key")
	}

	user, err := userRepo.FindByID(apiKey.UserID)
	if err != nil {
		return c.AbortUnauthorized("invalid API key")
	}
	if !user.Active {
		return c.AbortForbidden("account is disabled")
	}
	if !IPAllowed(apiKey.AllowedIPs, c.RealIP()) {
		return c.AbortForbidden("IP address not allowed for this API key")
	}

	scopes := []string(apiKey.Scopes)
	if len(scopes) == 0 {
		scopes = []string{models.ScopeSend}
	}

	c.Set(CtxUserID, int(apiKey.UserID))
	c.Set(CtxUserEmail, user.Email)
	c.Set(CtxUserRole, string(user.Role))
	c.Set(CtxAuthMethod, AuthMethodAPIKey)
	c.Set(CtxAPIKeyID, int(apiKey.ID))
	c.Set(CtxAPIKeyScopes, strings.Join(scopes, ","))
	if apiKey.WorkspaceID != nil {
		// The workspace middleware resolves the tenant from this binding, and legacy API-key routes rely on it.
		c.Set(CtxWorkspaceID, int(*apiKey.WorkspaceID))
		c.Set(CtxAPIKeyWorkspaceID, int(*apiKey.WorkspaceID))
	}

	if keyRepo != nil {
		go func() { _ = keyRepo.UpdateLastUsed(apiKey.ID) }()
	}

	return c.Next()
}

// An empty allowlist permits any IP; entries may be exact IPs or CIDR ranges.
func IPAllowed(allowed []string, clientIP string) bool {
	if len(allowed) == 0 {
		return true
	}
	ip := net.ParseIP(clientIP)
	for _, a := range allowed {
		if a == clientIP {
			return true
		}
		if strings.Contains(a, "/") {
			if _, network, err := net.ParseCIDR(a); err == nil && ip != nil && network.Contains(ip) {
				return true
			}
		}
	}
	return false
}

func hasScope(c *okapi.Context, scope string) bool {
	for _, s := range strings.Split(c.GetString(CtxAPIKeyScopes), ",") {
		if s == models.ScopeAll || s == scope {
			return true
		}
	}
	return false
}

// tenantAdminPaths demand ScopeAdmin for any method, so a content key cannot mint credentials or invite members.
var tenantAdminPaths = []string{
	"/api-keys",
	"/members",
	"/invitations",
	"/settings",
	"/sso",
	"/plan",
	"/forms",
	"/message-filters",
	"/teams",
	"/gdpr/",
}

// `send` grants none of read/write/admin, so a send-only key is confined to the public sending API.
func workspaceScopeFor(method, path string) string {
	for _, p := range tenantAdminPaths {
		if strings.Contains(path, p) {
			return models.ScopeAdmin
		}
	}
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return models.ScopeRead
	default:
		return models.ScopeWrite
	}
}

// Run inside workspace resolution so every workspace route is scope-checked and a new group cannot fail open.
func requireWorkspaceScope(c *okapi.Context) error {
	if c.GetString(CtxAuthMethod) != AuthMethodAPIKey {
		return nil
	}
	required := workspaceScopeFor(c.Request().Method, c.Request().URL.Path)
	if hasScope(c, required) {
		return nil
	}
	return c.AbortForbidden("API key missing required scope: " + required)
}

// Session/JWT callers pass through: workspace RBAC governs their access instead.
func RequireScope(scope string) okapi.Middleware {
	return func(c *okapi.Context) error {
		if c.GetString(CtxAuthMethod) != AuthMethodAPIKey {
			return c.Next()
		}
		if hasScope(c, scope) {
			return c.Next()
		}
		return c.AbortForbidden("API key missing required scope: " + scope)
	}
}

func RequireVerifiedEmail(verifier *emailverify.Service, userRepo *repositories.UserRepository) okapi.Middleware {
	return func(c *okapi.Context) error {
		if verifier == nil || !verifier.Required() {
			return c.Next()
		}
		userID := c.GetInt(CtxUserID)
		if userID == 0 {
			return c.Next()
		}
		user, err := userRepo.FindByID(uint(userID))
		if err != nil {
			return c.AbortUnauthorized("user not found")
		}
		if user.EmailVerifiedAt == nil {
			return c.AbortForbidden("email address is not verified")
		}
		return c.Next()
	}
}

func LoginRateLimitMiddleware(limiter *ratelimit.RedisLimiter) okapi.Middleware {
	return func(c *okapi.Context) error {
		if err := limiter.AllowLogin(c.Request().Context(), c.RealIP()); err != nil {
			return c.AbortTooManyRequests(err.Error())
		}
		return c.Next()
	}
}

func UserID(c *okapi.Context) uint { return uint(c.GetInt(CtxUserID)) }

func AuthMethod(c *okapi.Context) string { return c.GetString(CtxAuthMethod) }

func IsAPIKey(c *okapi.Context) bool { return AuthMethod(c) == AuthMethodAPIKey }

func APIKeyID(c *okapi.Context) uint { return uint(c.GetInt(CtxAPIKeyID)) }

// APIKeyWorkspaceID is nil for an account-wide key or a non-API-key caller.
func APIKeyWorkspaceID(c *okapi.Context) *uint {
	if id := c.GetInt(CtxAPIKeyWorkspaceID); id > 0 {
		v := uint(id)
		return &v
	}
	return nil
}

func APIKeyScopes(c *okapi.Context) []string {
	raw := c.GetString(CtxAPIKeyScopes)
	if raw == "" {
		return nil
	}
	return strings.Split(raw, ",")
}
