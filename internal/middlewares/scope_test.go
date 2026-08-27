// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/goposta/posta/internal/models"
	"github.com/jkaninda/okapi"
)

// stashAuth mimics authenticateAPIKey populating the auth context.
func stashAuth(method, scopes string) okapi.Middleware {
	return func(c *okapi.Context) error {
		c.Set(CtxAuthMethod, method)
		c.Set(CtxAPIKeyScopes, scopes)
		return c.Next()
	}
}

func TestRequireScope(t *testing.T) {
	cases := []struct {
		name       string
		authMethod string
		keyScopes  string
		required   string
		wantStatus int
	}{
		{"exact match allowed", AuthMethodAPIKey, "read", models.ScopeRead, http.StatusOK},
		{"wildcard allowed", AuthMethodAPIKey, "*", models.ScopeWebhooks, http.StatusOK},
		{"one of many allowed", AuthMethodAPIKey, "send,read", models.ScopeRead, http.StatusOK},
		{"missing scope forbidden", AuthMethodAPIKey, "send", models.ScopeRead, http.StatusForbidden},
		{"empty scopes forbidden for read", AuthMethodAPIKey, "", models.ScopeRead, http.StatusForbidden},
		{"send-only forbidden for webhooks", AuthMethodAPIKey, "send", models.ScopeWebhooks, http.StatusForbidden},

		// Scopes describe API keys only. A dashboard session carries none, and is
		// governed by workspace RBAC instead, so RequireScope must let it through.
		{"jwt session bypasses scopes", AuthMethodJWT, "", models.ScopeWebhooks, http.StatusOK},

		// write and admin are ordinary workspace scopes: the wildcard covers them.
		{"wildcard grants admin", AuthMethodAPIKey, "*", models.ScopeAdmin, http.StatusOK},
		{"explicit admin scope allowed", AuthMethodAPIKey, "admin", models.ScopeAdmin, http.StatusOK},
		{"read-only denied write", AuthMethodAPIKey, "read", models.ScopeWrite, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := okapi.NewTestServer(t)
			o.Get("/guarded", func(c *okapi.Context) error {
				return c.String(http.StatusOK, "ok")
			}).Use(stashAuth(tc.authMethod, tc.keyScopes), RequireScope(tc.required))

			req := httptest.NewRequest(http.MethodGet, "/guarded", nil)
			rec := httptest.NewRecorder()
			o.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("auth=%s scopes=%q require=%q: status = %d, want %d",
					tc.authMethod, tc.keyScopes, tc.required, rec.Code, tc.wantStatus)
			}
		})
	}
}
