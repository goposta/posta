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

	"github.com/goposta/posta/internal/models"
	"github.com/jkaninda/okapi"
)

// stashAPIKey mimics authenticateAPIKey's context for a workspace-bound key.
// boundWorkspace of 0 means an account-wide key.
func stashAPIKey(userID, boundWorkspace int, scopes string) okapi.Middleware {
	return func(c *okapi.Context) error {
		c.Set(CtxUserID, userID)
		c.Set(CtxAuthMethod, AuthMethodAPIKey)
		c.Set(CtxAPIKeyScopes, scopes)
		if boundWorkspace > 0 {
			c.Set(CtxWorkspaceID, boundWorkspace)
			c.Set(CtxAPIKeyWorkspaceID, boundWorkspace)
		}
		return c.Next()
	}
}

// stashJWT mimics okapi's ForwardClaims for a session caller.
func stashJWT(userID int) okapi.Middleware {
	return func(c *okapi.Context) error {
		c.Set(CtxUserID, userID)
		c.Set(CtxAuthMethod, AuthMethodJWT)
		return c.Next()
	}
}

// A workspace-bound API key needs no repository lookup: the binding itself is
// the authority, so these cases exercise resolveWorkspace with nil repos.
func TestRequireWorkspace_BoundAPIKey(t *testing.T) {
	cases := []struct {
		name       string
		header     string
		wantStatus int
		wantWSID   string
		wantRole   string
	}{
		{
			name:       "binding supplies the workspace when no header is sent",
			header:     "",
			wantStatus: http.StatusOK,
			wantWSID:   "7",
			wantRole:   string(models.WorkspaceRoleOwner),
		},
		{
			name:       "header agreeing with the binding is accepted",
			header:     "7",
			wantStatus: http.StatusOK,
			wantWSID:   "7",
			wantRole:   string(models.WorkspaceRoleOwner),
		},
		{
			name:       "header naming another workspace is refused",
			header:     "9",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "unparseable header is a bad request",
			header:     "abc",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := okapi.NewTestServer(t)
			o.Get("/ws", func(c *okapi.Context) error {
				return c.JSON(http.StatusOK, okapi.M{
					"workspace_id": c.GetInt(CtxWorkspaceID),
					"role":         c.GetString(CtxWorkspaceRole),
				})
			}).Use(stashAPIKey(1, 7, models.ScopeRead), RequireWorkspaceMiddleware(nil, nil))

			req := httptest.NewRequest(http.MethodGet, "/ws", nil)
			if tc.header != "" {
				req.Header.Set(WorkspaceHeader, tc.header)
			}
			rec := httptest.NewRecorder()
			o.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantStatus != http.StatusOK {
				return
			}
			if body := rec.Body.String(); !strings.Contains(body, `"workspace_id":`+tc.wantWSID) || !strings.Contains(body, `"role":"`+tc.wantRole+`"`) {
				t.Errorf("body = %s, want workspace %s as %s", body, tc.wantWSID, tc.wantRole)
			}
		})
	}
}

// route registers a handler for an arbitrary method and returns the *Route, so
// middleware can be attached (okapi's Handle returns nothing).
func route(o *okapi.TestServer, method, path string, h okapi.HandlerFunc) *okapi.Route {
	switch method {
	case http.MethodGet:
		return o.Get(path, h)
	case http.MethodPost:
		return o.Post(path, h)
	case http.MethodPut:
		return o.Put(path, h)
	case http.MethodDelete:
		return o.Delete(path, h)
	default:
		panic("unsupported method " + method)
	}
}

// Scopes bound what a key may do inside the workspace it is bound to. The
// binding alone must never imply full access: a send-only key is confined to the
// public sending API and must not read or modify workspace resources.
func TestWorkspaceScopeEnforcement(t *testing.T) {
	cases := []struct {
		name       string
		scopes     string
		method     string
		path       string
		wantStatus int
	}{
		// The vulnerability this guards: `send` is the default scope for a key
		// created without an explicit set, so a leak of the weakest key must not
		// expose the workspace's contacts, templates or domains.
		{"send cannot read", models.ScopeSend, http.MethodGet, "/templates", http.StatusForbidden},
		{"send cannot write", models.ScopeSend, http.MethodPost, "/templates", http.StatusForbidden},
		{"send cannot administer", models.ScopeSend, http.MethodGet, "/api-keys", http.StatusForbidden},

		{"read may read", models.ScopeRead, http.MethodGet, "/templates", http.StatusOK},
		{"read may not write", models.ScopeRead, http.MethodPost, "/templates", http.StatusForbidden},
		{"read may not delete", models.ScopeRead, http.MethodDelete, "/templates/1", http.StatusForbidden},

		{"write may write", models.ScopeWrite, http.MethodPost, "/templates", http.StatusOK},
		{"write may delete", models.ScopeWrite, http.MethodDelete, "/templates/1", http.StatusOK},
		// write covers content, not the tenant itself — minting credentials or
		// inviting members stays behind admin.
		{"write may not mint keys", models.ScopeWrite, http.MethodPost, "/api-keys", http.StatusForbidden},
		{"write may not invite", models.ScopeWrite, http.MethodPost, "/invitations", http.StatusForbidden},
		{"write may not change settings", models.ScopeWrite, http.MethodPut, "/settings", http.StatusForbidden},

		{"admin may mint keys", models.ScopeAdmin, http.MethodPost, "/api-keys", http.StatusOK},
		{"admin may manage members", models.ScopeAdmin, http.MethodGet, "/members", http.StatusOK},
		// admin is tenant administration, not a superset of content access.
		{"admin alone cannot read templates", models.ScopeAdmin, http.MethodGet, "/templates", http.StatusForbidden},

		{"wildcard grants everything", models.ScopeAll, http.MethodDelete, "/templates/1", http.StatusOK},
		{"wildcard may administer", models.ScopeAll, http.MethodPost, "/api-keys", http.StatusOK},

		{"multiple scopes combine", "read,write", http.MethodPost, "/templates", http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := okapi.NewTestServer(t)
			route(o, tc.method, "/api/v1/workspaces/current"+tc.path, func(c *okapi.Context) error {
				return c.String(http.StatusOK, "ok")
			}).Use(stashAPIKey(1, 7, tc.scopes), RequireWorkspaceMiddleware(nil, nil))

			req := httptest.NewRequest(tc.method, "/api/v1/workspaces/current"+tc.path, nil)
			rec := httptest.NewRecorder()
			o.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("scopes=%q %s %s: status = %d, want %d",
					tc.scopes, tc.method, tc.path, rec.Code, tc.wantStatus)
			}
		})
	}
}

// A dashboard session carries no scopes; workspace RBAC governs it instead.
func TestWorkspaceScope_SessionUnaffected(t *testing.T) {
	o := okapi.NewTestServer(t)
	o.Get("/api/v1/workspaces/current/api-keys", func(c *okapi.Context) error {
		return c.String(http.StatusOK, "ok")
	}).Use(stashJWT(1), WorkspaceFromQueryOrHeader(nil, nil))

	rec := httptest.NewRecorder()
	o.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/current/api-keys", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (a session must not be scope checked)", rec.Code)
	}
}

func TestRequireWorkspace_RejectsUnauthenticated(t *testing.T) {
	o := okapi.NewTestServer(t)
	o.Get("/ws", func(c *okapi.Context) error {
		return c.String(http.StatusOK, "ok")
	}).Use(RequireWorkspaceMiddleware(nil, nil))

	rec := httptest.NewRecorder()
	o.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ws", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// A session caller must name a workspace on a required-workspace route; there is
// no binding to fall back on.
func TestRequireWorkspace_JWTWithoutHeader(t *testing.T) {
	o := okapi.NewTestServer(t)
	o.Get("/ws", func(c *okapi.Context) error {
		return c.String(http.StatusOK, "ok")
	}).Use(stashJWT(1), RequireWorkspaceMiddleware(nil, nil))

	rec := httptest.NewRecorder()
	o.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ws", nil))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// WorkspaceFromQueryOrHeader exists for EventSource and download links, which
// cannot set headers. A bound key still overrides whatever the query claims.
func TestWorkspaceFromQuery_BoundKeyIgnoresMismatchedQuery(t *testing.T) {
	o := okapi.NewTestServer(t)
	o.Get("/stream", func(c *okapi.Context) error {
		return c.String(http.StatusOK, "ok")
	}).Use(stashAPIKey(1, 7, models.ScopeRead), WorkspaceFromQueryOrHeader(nil, nil))

	rec := httptest.NewRecorder()
	o.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/stream?workspace_id=9", nil))

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}
