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

// Handlers are never invoked, only each route's middleware chain, so a zero-value handler set suffices.
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

// Summary is unique across the workspace surface and reads better in failures than a method/path pair.
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

func seedSessionRole(role models.WorkspaceRole) okapi.Middleware {
	return func(c *okapi.Context) error {
		c.Set(middlewares.CtxUserID, 1)
		c.Set(middlewares.CtxAuthMethod, middlewares.AuthMethodJWT)
		c.Set(middlewares.CtxWorkspaceID, 1)
		c.Set(middlewares.CtxWorkspaceRole, string(role))
		return c.Next()
	}
}

// 200 means the route's own middleware chain let the request through to the handler.
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

// Regression guard: these routes claimed admin/owner only but had no role guard, so even a viewer reached them.
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

// The route claimed owner-only, but an admin could delete the workspace too.
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

// The RBAC fix must not lock ordinary members out of the reads they legitimately need.
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
