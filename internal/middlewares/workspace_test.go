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
func stashAPIKey(userID, boundWorkspace int) okapi.Middleware {
	return func(c *okapi.Context) error {
		c.Set(CtxUserID, userID)
		c.Set(CtxAuthMethod, AuthMethodAPIKey)
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
			}).Use(stashAPIKey(1, 7), RequireWorkspaceMiddleware(nil, nil))

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
	}).Use(stashAPIKey(1, 7), WorkspaceFromQueryOrHeader(nil, nil))

	rec := httptest.NewRecorder()
	o.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/stream?workspace_id=9", nil))

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}
