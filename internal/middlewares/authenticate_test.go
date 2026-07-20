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
	"strings"
	"testing"
	"time"

	"github.com/goposta/posta/internal/config"
	"github.com/jkaninda/okapi"
)

const testSecret = "test-secret-value"

func testJWT(t *testing.T, role string) string {
	t.Helper()
	tok, err := okapi.GenerateJwtToken([]byte(testSecret), map[string]any{
		"sub":   1,
		"email": "u@example.com",
		"role":  role,
		"aud":   "posta",
		"jti":   "jti-1",
	}, time.Hour)
	if err != nil {
		t.Fatalf("GenerateJwtToken: %v", err)
	}
	return tok
}

// jwtOnlyServer mounts Authenticate with nil API-key collaborators. That is safe
// precisely because a credential without the psk_ prefix must never reach the
// API-key path — if it did, these tests would panic rather than quietly pass.
func jwtOnlyServer(t *testing.T) *okapi.TestServer {
	t.Helper()
	cfg := &config.Config{JWTSecret: testSecret}
	o := okapi.NewTestServer(t)
	o.Get("/protected", func(c *okapi.Context) error {
		return c.String(http.StatusOK, c.GetString(CtxAuthMethod))
	}).Use(Authenticate(JWTAuth(cfg), nil, nil, nil))
	return o
}

func TestAuthenticate_JWT(t *testing.T) {
	token := testJWT(t, "user")

	cases := []struct {
		name       string
		apply      func(*http.Request)
		wantStatus int
	}{
		{
			name:       "authorization header",
			apply:      func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+token) },
			wantStatus: http.StatusOK,
		},
		{
			name:       "session cookie",
			apply:      func(r *http.Request) { r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token}) },
			wantStatus: http.StatusOK,
		},
		{
			name:       "no credential",
			apply:      func(*http.Request) {},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "garbage bearer token",
			apply:      func(r *http.Request) { r.Header.Set("Authorization", "Bearer not-a-jwt") },
			wantStatus: http.StatusUnauthorized,
		},
		{
			// The whole point of dropping query:token — a JWT in the URL must not
			// authenticate, even though it is otherwise a perfectly valid token.
			name:       "valid jwt in query is refused",
			apply:      func(r *http.Request) { r.URL.RawQuery = "token=" + token },
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := jwtOnlyServer(t)
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			tc.apply(req)
			rec := httptest.NewRecorder()
			o.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantStatus == http.StatusOK && rec.Body.String() != AuthMethodJWT {
				t.Errorf("auth_method = %q, want %q", rec.Body.String(), AuthMethodJWT)
			}
		})
	}
}

// An expired or revoked cookie must not fall through to "authentication
// required" — it has to be rejected as an invalid session.
func TestAuthenticate_RejectsExpiredCookie(t *testing.T) {
	expired, err := okapi.GenerateJwtToken([]byte(testSecret), map[string]any{
		"sub": 1, "aud": "posta", "jti": "jti-1",
	}, -time.Hour)
	if err != nil {
		t.Fatalf("GenerateJwtToken: %v", err)
	}

	o := jwtOnlyServer(t)
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: expired})
	rec := httptest.NewRecorder()
	o.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// A token with no jti cannot be checked against the revocation list, so it is
// not a usable session. It must read as unauthenticated (401), not as a
// permissions problem (403) — okapi's default for a ValidateClaims failure.
func TestAuthenticate_RejectsTokenWithoutJTI(t *testing.T) {
	noJTI, err := okapi.GenerateJwtToken([]byte(testSecret), map[string]any{
		"sub": 1, "aud": "posta", "role": "user",
	}, time.Hour)
	if err != nil {
		t.Fatalf("GenerateJwtToken: %v", err)
	}

	o := jwtOnlyServer(t)
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: noJTI})
	rec := httptest.NewRecorder()
	o.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// A psk_ credential must never be parsed as a JWT, wherever it arrives. With nil
// API-key collaborators the API-key path panics, which is what we assert: it
// proves dispatch reached that branch rather than silently 401ing as a bad JWT.
func TestAuthenticate_RoutesAPIKeyPrefixToKeyPath(t *testing.T) {
	for _, tc := range []struct {
		name  string
		apply func(*http.Request)
	}{
		{"header", func(r *http.Request) { r.Header.Set("Authorization", "Bearer psk_deadbeef") }},
		{"bare header", func(r *http.Request) { r.Header.Set("Authorization", "psk_deadbeef") }},
		{"query", func(r *http.Request) { r.URL.RawQuery = "token=psk_deadbeef" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("psk_ credential did not reach the API-key path")
				}
			}()

			cfg := &config.Config{JWTSecret: testSecret}
			mw := Authenticate(JWTAuth(cfg), nil, nil, nil)
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			tc.apply(req)

			o := okapi.NewTestServer(t)
			c := okapi.NewContext(o.Okapi, httptest.NewRecorder(), req)
			_ = mw(c)
		})
	}
}

// A JWT is not an admin credential unless its role claim says so.
func TestJWTAdminAuth_RejectsNonAdmin(t *testing.T) {
	cfg := &config.Config{JWTSecret: testSecret}

	for _, tc := range []struct {
		role       string
		wantStatus int
	}{
		{"admin", http.StatusOK},
		{"user", http.StatusForbidden},
	} {
		t.Run(tc.role, func(t *testing.T) {
			adminAuth := JWTAdminAuth(cfg)
			o := okapi.NewTestServer(t)
			o.Get("/admin", func(c *okapi.Context) error {
				return c.String(http.StatusOK, "ok")
			}).Use(adminAuth.Middleware)

			req := httptest.NewRequest(http.MethodGet, "/admin", nil)
			req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: testJWT(t, tc.role)})
			rec := httptest.NewRecorder()
			o.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("role %q: status = %d, want %d", tc.role, rec.Code, tc.wantStatus)
			}
		})
	}
}

// The lookup order and contents are load-bearing: header for CLI/SDK, cookie for
// the browser, and no query source at all.
func TestTokenLookup(t *testing.T) {
	if want := "header:Authorization,cookie:posta_session"; tokenLookup != want {
		t.Errorf("tokenLookup = %q, want %q", tokenLookup, want)
	}
	if strings.Contains(tokenLookup, "query") {
		t.Error("a user JWT must never be accepted from the query string")
	}
}
