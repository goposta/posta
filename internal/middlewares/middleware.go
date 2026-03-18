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

package middlewares

import (
	"errors"
	"net"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jkaninda/okapi"
	"github.com/jkaninda/posta/internal/config"
	"github.com/jkaninda/posta/internal/services/auth"
	"github.com/jkaninda/posta/internal/services/ratelimit"
	sessionpkg "github.com/jkaninda/posta/internal/services/session"
	"github.com/jkaninda/posta/internal/storage/repositories"
)

func baseJWTAuth(cfg *config.Config) okapi.JWTAuth {
	return okapi.JWTAuth{
		SigningSecret: []byte(cfg.JWTSecret),
		Audience:      "posta",
		ContextKey:    "jwt_user",
		ForwardClaims: map[string]string{
			"user_id": "sub",
			"email":   "email",
			"role":    "role",
			"jti":     "jti",
		},
	}
}

// sessionValidator returns a ValidateClaims function that checks the Redis blacklist.
func sessionValidator(store *sessionpkg.Store) func(c *okapi.Context, claims jwt.Claims) error {
	return func(c *okapi.Context, claims jwt.Claims) error {
		mapClaims, ok := claims.(jwt.MapClaims)
		if !ok {
			return nil // no claims to check
		}
		jti, _ := mapClaims["jti"].(string)
		if jti == "" {
			return nil // legacy token without jti, allow through
		}
		if store.IsRevoked(c.Request().Context(), jti) {
			return errors.New("session has been revoked")
		}
		return nil
	}
}

// JWTAuth creates user JWT auth middleware. If sessionStore is non-nil, revoked sessions are rejected.
func JWTAuth(cfg *config.Config, sessionStore ...*sessionpkg.Store) okapi.JWTAuth {
	auth := baseJWTAuth(cfg)
	if len(sessionStore) > 0 && sessionStore[0] != nil {
		auth.ValidateClaims = sessionValidator(sessionStore[0])
	}
	return auth
}

// JWTAdminAuth creates admin JWT auth middleware.
func JWTAdminAuth(cfg *config.Config, sessionStore ...*sessionpkg.Store) okapi.JWTAuth {
	auth := baseJWTAuth(cfg)
	auth.ClaimsExpression = "Equals(`role`,`admin`)"
	if len(sessionStore) > 0 && sessionStore[0] != nil {
		auth.ValidateClaims = sessionValidator(sessionStore[0])
	}
	return auth
}

// JWTAdminQueryAuth creates admin JWT auth via query param (for SSE).
func JWTAdminQueryAuth(cfg *config.Config, sessionStore ...*sessionpkg.Store) okapi.JWTAuth {
	auth := baseJWTAuth(cfg)
	auth.ClaimsExpression = "Equals(`role`,`admin`)"
	auth.TokenLookup = "query:token"
	if len(sessionStore) > 0 && sessionStore[0] != nil {
		auth.ValidateClaims = sessionValidator(sessionStore[0])
	}
	return auth
}

// LoginRateLimitMiddleware limits login attempts per IP address using Redis.
func LoginRateLimitMiddleware(limiter *ratelimit.RedisLimiter) okapi.Middleware {
	return func(c *okapi.Context) error {
		ip := c.RealIP()
		if err := limiter.AllowLogin(c.Request().Context(), ip); err != nil {
			return c.AbortTooManyRequests(err.Error())
		}
		return c.Next()

	}
}

// APIKeyAuthMiddleware validates API key from Authorization header and sets user context.
func APIKeyAuthMiddleware(keyService *auth.APIKeyService, userRepo *repositories.UserRepository, keyRepo *repositories.APIKeyRepository) okapi.Middleware {
	return func(c *okapi.Context) error {
		authHeader := c.Header("Authorization")
		if authHeader == "" {
			return c.AbortUnauthorized("missing Authorization header")
		}

		rawKey := strings.TrimPrefix(authHeader, "Bearer ")
		if rawKey == authHeader {
			return c.AbortUnauthorized("invalid Authorization format, expected: Bearer <API_KEY>")
		}

		apiKey, err := keyService.ValidateKey(rawKey)
		if err != nil {
			return c.AbortUnauthorized(err.Error())
		}

		user, err := userRepo.FindByID(apiKey.UserID)
		if err != nil {
			return c.AbortUnauthorized("user not found")
		}

		if !user.Active {
			return c.AbortForbidden("account is disabled")
		}

		// Check IP allowlist
		if len(apiKey.AllowedIPs) > 0 {
			clientIP := c.RealIP()
			allowed := false
			for _, ip := range apiKey.AllowedIPs {
				if ip == clientIP {
					allowed = true
					break
				}
				// Support CIDR notation
				if strings.Contains(ip, "/") {
					_, network, err := net.ParseCIDR(ip)
					if err == nil && network.Contains(net.ParseIP(clientIP)) {
						allowed = true
						break
					}
				}
			}
			if !allowed {
				return c.AbortForbidden("IP address not allowed for this API key")
			}
		}

		c.Set("user_id", int(apiKey.UserID))
		c.Set("api_key_id", int(apiKey.ID))
		c.Set("user_email", user.Email)

		go func() { _ = keyRepo.UpdateLastUsed(apiKey.ID) }()

		return c.Next()

	}
}
