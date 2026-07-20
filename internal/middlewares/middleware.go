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

// Package middlewares holds HTTP middleware: authentication, workspace scoping,
// and role-based access control.
//
// Two credentials reach the API:
//
//   - A user JWT, presented by the dashboard as an HttpOnly session cookie or by
//     CLI/SDK clients as `Authorization: Bearer <jwt>`.
//   - An API key (psk_…), presented as `Authorization: Bearer psk_…` or, for
//     browser-direct streams that cannot set headers, as `?token=psk_…`.
//
// Authenticate accepts either. APIKeyAuth accepts only the latter, and the
// admin JWT middleware only the former.
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

// Context keys populated by these middlewares.
const (
	CtxUserID    = "user_id"
	CtxUserEmail = "user_email"
	// CtxUserRole is the platform role ("admin" | "user"). For JWT callers okapi
	// forwards it from the token; for API-key callers it is read from the owner.
	CtxUserRole = "role"
	CtxJTI      = "jti"
	// CtxAuthMethod records how the request authenticated, so downstream
	// middleware (notably RequireScope) can tell a session from a machine key.
	CtxAuthMethod    = "auth_method"
	CtxWorkspaceID   = "workspace_id"
	CtxWorkspaceRole = "workspace_role"
	CtxAPIKeyID      = "api_key_id"
	CtxAPIKeyScopes  = "api_key_scopes"
	// CtxAPIKeyWorkspaceID holds the workspace an API key is bound to, when the
	// presented key is workspace-scoped (absent for account-wide keys).
	CtxAPIKeyWorkspaceID = "api_key_workspace_id"
)

// Values stored under CtxAuthMethod.
const (
	AuthMethodJWT    = "jwt"
	AuthMethodAPIKey = "api_key"
)

// SessionCookieName re-exports the browser session cookie name, so route and
// middleware code need not reach into the session package for it.
const SessionCookieName = sessionpkg.CookieName

// tokenLookup resolves the JWT from the Authorization header (CLI, SDKs) and
// then the browser session cookie. It deliberately omits `query:token`: browsers
// send cookies on EventSource handshakes and CLIs set the header, so a user JWT
// never has to travel in a URL — where it would land in proxy logs, browser
// history, and Referer headers.
const tokenLookup = "header:Authorization,cookie:" + SessionCookieName

// ctxRejection records why claim validation failed, so OnUnauthorized can answer
// with the right status. okapi routes every failure — a missing token, an expired
// one, and a valid token whose claims fall short — through that single hook, so
// without this marker "you are not an admin" would be indistinguishable from "you
// are not logged in", and the dashboard would sign the user out on a 403.
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
		// Deliberately not ClaimsExpression: okapi evaluates it before
		// ValidateClaims, which would deny us the chance to tag the rejection.
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

// validateClaims runs only once okapi has accepted the token's signature and
// expiry, so reaching it means the caller holds a genuine session.
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

// JWTAuth builds user JWT auth (Authorization header or session cookie).
// Revoked sessions are rejected when sessionStore is provided.
func JWTAuth(cfg *config.Config, sessionStore ...*sessionpkg.Store) okapi.JWTAuth {
	return baseJWTAuth(cfg, firstStore(sessionStore), false)
}

// JWTAdminAuth builds admin-only JWT auth. An API key is never an admin
// credential: a psk_ token presented here fails JWT parsing and is rejected.
func JWTAdminAuth(cfg *config.Config, sessionStore ...*sessionpkg.Store) okapi.JWTAuth {
	return baseJWTAuth(cfg, firstStore(sessionStore), true)
}

func firstStore(stores []*sessionpkg.Store) *sessionpkg.Store {
	if len(stores) > 0 {
		return stores[0]
	}
	return nil
}

// Authenticate accepts either a user JWT or an API key.
//
// API keys (psk_…) are read from the Authorization header or the `?token=` query
// param — the latter so browser-direct streams (EventSource, download links) and
// machine clients can authenticate without headers. Any other credential is
// treated as a JWT and resolved by okapi from the header or the session cookie.
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
			// A browser presents the JWT only as an HttpOnly cookie, which never
			// shows up in the header or query. Let okapi read it via TokenLookup.
			if _, err := c.Cookie(SessionCookieName); err != nil {
				return c.AbortUnauthorized("authentication required")
			}
		}
		c.Set(CtxAuthMethod, AuthMethodJWT)
		return jwtAuth.Middleware(c)
	}
}

// APIKeyAuth accepts only an API key, and only from the Authorization header. It
// guards the public machine-facing API (/api/v1/emails/*, /webhooks, …), where a
// browser session is not a valid credential and no caller needs the key in a URL.
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

// bearerToken returns the Authorization header's token, stripping an optional
// `Bearer ` prefix so a bare `Authorization: psk_…` still works.
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
	if !ipAllowed(apiKey.AllowedIPs, c.RealIP()) {
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
		// Tenant resolution proper happens in the workspace middleware; this is
		// the binding it reads, and what the legacy API-key routes rely on.
		c.Set(CtxWorkspaceID, int(*apiKey.WorkspaceID))
		c.Set(CtxAPIKeyWorkspaceID, int(*apiKey.WorkspaceID))
	}

	if keyRepo != nil {
		go func() { _ = keyRepo.UpdateLastUsed(apiKey.ID) }()
	}

	return c.Next()
}

// ipAllowed reports whether clientIP is permitted by an API key's allowlist. An
// empty allowlist permits any IP; entries may be exact IPs or CIDR ranges.
func ipAllowed(allowed []string, clientIP string) bool {
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

// hasScope reports whether the resolved API key grants scope ("*" grants all).
func hasScope(c *okapi.Context, scope string) bool {
	for _, s := range strings.Split(c.GetString(CtxAPIKeyScopes), ",") {
		if s == models.ScopeAll || s == scope {
			return true
		}
	}
	return false
}

// tenantAdminPaths are workspace path segments whose routes administer the
// tenant itself — credentials, membership, billing, and configuration — rather
// than its content. They demand ScopeAdmin whatever the method, so a key that
// can manage templates cannot also mint credentials or invite members.
var tenantAdminPaths = []string{
	"/api-keys",
	"/members",
	"/invitations",
	"/settings",
	"/sso",
	"/plan",
}

// workspaceScopeFor returns the scope a workspace-scoped request demands.
// Reads need `read`, mutations need `write`, and tenant administration needs
// `admin`. Note that `send` grants none of these: sending lives on the public
// API (/api/v1/emails/*), so a send-only key is confined to it and cannot read
// or modify workspace resources.
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

// requireWorkspaceScope enforces workspaceScopeFor on API-key callers. Session
// callers pass through — their access is governed by workspace RBAC instead.
//
// This runs inside workspace resolution rather than as a route-group middleware
// so that it cannot be forgotten: any route that resolves a workspace is scope
// checked by construction, and a new group cannot silently fail open.
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

// RequireScope guards a route for API-key callers, demanding the presented key
// carry the given scope (or "*"). Session/JWT callers pass through — their access
// is governed by workspace RBAC instead.
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

// RequireVerifiedEmail blocks actions that are risky when the caller's email
// address has not been confirmed.
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

// LoginRateLimitMiddleware limits login attempts per IP address using Redis.
func LoginRateLimitMiddleware(limiter *ratelimit.RedisLimiter) okapi.Middleware {
	return func(c *okapi.Context) error {
		if err := limiter.AllowLogin(c.Request().Context(), c.RealIP()); err != nil {
			return c.AbortTooManyRequests(err.Error())
		}
		return c.Next()
	}
}

// UserID returns the authenticated user id (0 if absent).
func UserID(c *okapi.Context) uint { return uint(c.GetInt(CtxUserID)) }

// AuthMethod returns how the request authenticated ("jwt" | "api_key" | "").
func AuthMethod(c *okapi.Context) string { return c.GetString(CtxAuthMethod) }

// IsAPIKey reports whether the caller authenticated with an API key.
func IsAPIKey(c *okapi.Context) bool { return AuthMethod(c) == AuthMethodAPIKey }

// APIKeyID returns the presented API key's id (0 for session/JWT callers).
func APIKeyID(c *okapi.Context) uint { return uint(c.GetInt(CtxAPIKeyID)) }

// APIKeyWorkspaceID returns the workspace an API key is bound to, or nil for an
// account-wide key or a non-API-key caller.
func APIKeyWorkspaceID(c *okapi.Context) *uint {
	if id := c.GetInt(CtxAPIKeyWorkspaceID); id > 0 {
		v := uint(id)
		return &v
	}
	return nil
}

// APIKeyScopes returns the presented key's scopes (nil for session/JWT callers).
func APIKeyScopes(c *okapi.Context) []string {
	raw := c.GetString(CtxAPIKeyScopes)
	if raw == "" {
		return nil
	}
	return strings.Split(raw, ",")
}
