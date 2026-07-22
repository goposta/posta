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

package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/goposta/posta/internal/config"
	"github.com/goposta/posta/internal/middlewares"
	"github.com/goposta/posta/internal/models"
	"github.com/jkaninda/okapi"
)

// workspaceRouteDefs builds the real route definitions. The handlers are never
// invoked — only the middleware chain each route carries is exercised — so the
// zero-value handler set is enough, and this stays a pure wiring test with no
// database or service dependencies.
func workspaceRouteDefs(t *testing.T) []okapi.RouteDefinition {
	t.Helper()

	o := okapi.NewTestServer(t)
	r := &Router{
		app: o.Okapi,
		cfg: &config.Config{},
		v1:  o.Group("/api/v1"),
	}
	return append(r.workspaceRoutes(), r.oauthRoutes()...)
}

// defBySummary indexes route definitions by their Summary, which is unique
// across the workspace surface and reads better in failure output than a
// method/path pair repeated across groups.
func defBySummary(t *testing.T, defs []okapi.RouteDefinition, summary string) okapi.RouteDefinition {
	t.Helper()

	for _, d := range defs {
		if d.Summary == summary {
			return d
		}
	}
	t.Fatalf("no route definition with summary %q", summary)
	return okapi.RouteDefinition{}
}

// seedSessionRole mimics what RequireWorkspaceMiddleware leaves in the context
// for a dashboard caller who is a member of the active workspace.
func seedSessionRole(role models.WorkspaceRole) okapi.Middleware {
	return func(c *okapi.Context) error {
		c.Set(middlewares.CtxUserID, 1)
		c.Set(middlewares.CtxAuthMethod, middlewares.AuthMethodJWT)
		c.Set(middlewares.CtxWorkspaceID, 1)
		c.Set(middlewares.CtxWorkspaceRole, string(role))
		return c.Next()
	}
}

// runRouteAs replays a route's own middleware chain against a caller holding
// the given workspace role and reports the resulting status. 200 means the
// chain let the request through to the handler.
func runRouteAs(t *testing.T, def okapi.RouteDefinition, role models.WorkspaceRole) int {
	t.Helper()

	chain := []okapi.Middleware{seedSessionRole(role)}
	for _, m := range def.Middlewares {
		if m != nil { // group-level middlewares are nil in this bare Router
			chain = append(chain, m)
		}
	}

	o := okapi.NewTestServer(t)
	o.Get("/t", func(c *okapi.Context) error {
		return c.String(http.StatusOK, "reached handler")
	}, okapi.UseMiddleware(chain...))

	rec := httptest.NewRecorder()
	o.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/t", nil))
	return rec.Code
}

// TestWorkspaceAdminRoutesRejectNonAdmins is the regression guard for the
// missing-RBAC finding: these routes documented themselves as admin/owner only
// while carrying no role guard at all, so any member — including a viewer —
// reached the handler.
func TestWorkspaceAdminRoutesRejectNonAdmins(t *testing.T) {
	defs := workspaceRouteDefs(t)

	adminOnly := []string{
		"Update workspace",
		"Update member role",
		"Remove member",
		"Invite member",
		"List pending invitations",
		"Cancel invitation",
		"Export workspace data",
		"Import workspace data",
		"Get workspace SSO config",
		"Set workspace SSO config",
		"Delete workspace SSO config",
	}

	for _, summary := range adminOnly {
		t.Run(summary, func(t *testing.T) {
			def := defBySummary(t, defs, summary)

			for _, role := range []models.WorkspaceRole{models.WorkspaceRoleOwner, models.WorkspaceRoleAdmin} {
				if got := runRouteAs(t, def, role); got != http.StatusOK {
					t.Errorf("%s as %s: status = %d, want 200", summary, role, got)
				}
			}
			for _, role := range []models.WorkspaceRole{models.WorkspaceRoleEditor, models.WorkspaceRoleViewer} {
				if got := runRouteAs(t, def, role); got != http.StatusForbidden {
					t.Errorf("%s as %s: status = %d, want 403", summary, role, got)
				}
			}
		})
	}
}

// TestDeleteWorkspaceIsOwnerOnly pins the stricter tier: the route documented
// itself as owner-only, but an admin could delete the workspace too.
func TestDeleteWorkspaceIsOwnerOnly(t *testing.T) {
	def := defBySummary(t, workspaceRouteDefs(t), "Delete workspace")

	if got := runRouteAs(t, def, models.WorkspaceRoleOwner); got != http.StatusOK {
		t.Errorf("delete workspace as owner: status = %d, want 200", got)
	}
	for _, role := range []models.WorkspaceRole{
		models.WorkspaceRoleAdmin,
		models.WorkspaceRoleEditor,
		models.WorkspaceRoleViewer,
	} {
		if got := runRouteAs(t, def, role); got != http.StatusForbidden {
			t.Errorf("delete workspace as %s: status = %d, want 403", role, got)
		}
	}
}

// TestWorkspaceMemberRoutesStayOpenToMembers guards the other direction: the
// fix must not lock ordinary members out of the reads they legitimately need.
func TestWorkspaceMemberRoutesStayOpenToMembers(t *testing.T) {
	defs := workspaceRouteDefs(t)

	for _, summary := range []string{"Get current workspace", "List workspace members", "Get workspace plan"} {
		t.Run(summary, func(t *testing.T) {
			def := defBySummary(t, defs, summary)
			for _, role := range []models.WorkspaceRole{
				models.WorkspaceRoleOwner,
				models.WorkspaceRoleAdmin,
				models.WorkspaceRoleEditor,
				models.WorkspaceRoleViewer,
			} {
				if got := runRouteAs(t, def, role); got != http.StatusOK {
					t.Errorf("%s as %s: status = %d, want 200", summary, role, got)
				}
			}
		})
	}
}
