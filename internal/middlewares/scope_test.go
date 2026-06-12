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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/goposta/posta/internal/models"
	"github.com/jkaninda/okapi"
)

// stashScopes mimics APIKeyAuthMiddleware setting the key's scopes on context.
func stashScopes(raw string) okapi.Middleware {
	return func(c *okapi.Context) error {
		c.Set("api_key_scopes", raw)
		return c.Next()
	}
}

func TestRequireScope(t *testing.T) {
	cases := []struct {
		name       string
		keyScopes  string
		required   string
		wantStatus int
	}{
		{"exact match allowed", "read", models.ScopeRead, http.StatusOK},
		{"wildcard allowed", "*", models.ScopeWebhooks, http.StatusOK},
		{"one of many allowed", "send,read", models.ScopeRead, http.StatusOK},
		{"missing scope forbidden", "send", models.ScopeRead, http.StatusForbidden},
		{"empty scopes forbidden for read", "", models.ScopeRead, http.StatusForbidden},
		{"send-only forbidden for webhooks", "send", models.ScopeWebhooks, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := okapi.NewTestServer(t)
			o.Get("/guarded", func(c *okapi.Context) error {
				return c.String(http.StatusOK, "ok")
			}).Use(stashScopes(tc.keyScopes), RequireScope(tc.required))

			req := httptest.NewRequest(http.MethodGet, "/guarded", nil)
			rec := httptest.NewRecorder()
			o.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("scopes=%q require=%q: status = %d, want %d",
					tc.keyScopes, tc.required, rec.Code, tc.wantStatus)
			}
		})
	}
}
