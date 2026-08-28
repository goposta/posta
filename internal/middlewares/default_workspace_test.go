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

type stubResolver struct {
	ws     *models.Workspace
	role   models.WorkspaceRole
	err    error
	called int
}

func (s *stubResolver) DefaultWorkspace(uint) (*models.Workspace, models.WorkspaceRole, error) {
	s.called++
	return s.ws, s.role, s.err
}

func runFallback(t *testing.T, resolver DefaultWorkspaceResolver) (int, string, int) {
	t.Helper()
	app := okapi.New()

	var gotWorkspace int
	var gotRole string
	app.Get("/t", func(c *okapi.Context) error {
		gotWorkspace = c.GetInt(CtxWorkspaceID)
		gotRole = c.GetString(CtxWorkspaceRole)
		return c.JSON(http.StatusOK, okapi.M{"ok": true})
	}).Use(stashJWT(42), OptionalWorkspaceMiddleware(nil, resolver))

	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/t", nil))
	return gotWorkspace, gotRole, rec.Code
}

// The default workspace used to be the caller's personal one, which they always
// owned, so the fallback hard-coded owner. It can now be any workspace they
// belong to — including one where they are a viewer — and carrying owner through
// would grant write access on every header-less request.
func TestDefaultWorkspaceFallbackUsesMembershipRole(t *testing.T) {
	for _, role := range []models.WorkspaceRole{
		models.WorkspaceRoleViewer,
		models.WorkspaceRoleEditor,
		models.WorkspaceRoleAdmin,
		models.WorkspaceRoleOwner,
	} {
		t.Run(string(role), func(t *testing.T) {
			resolver := &stubResolver{ws: &models.Workspace{ID: 9}, role: role}
			ws, got, code := runFallback(t, resolver)

			if code != http.StatusOK {
				t.Fatalf("status = %d, want 200", code)
			}
			if ws != 9 {
				t.Fatalf("workspace = %d, want 9", ws)
			}
			if got != string(role) {
				t.Fatalf("role = %q, want %q", got, role)
			}
		})
	}
}

func TestDefaultWorkspaceFallbackViewerCannotEdit(t *testing.T) {
	resolver := &stubResolver{ws: &models.Workspace{ID: 9}, role: models.WorkspaceRoleViewer}
	_, role, _ := runFallback(t, resolver)

	if models.WorkspaceRole(role).CanEdit() {
		t.Fatal("a viewer resolved through the header-less fallback must not be able to edit")
	}
}

func TestDefaultWorkspaceFallbackWithNoMembership(t *testing.T) {
	resolver := &stubResolver{}
	ws, role, code := runFallback(t, resolver)

	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200: a user with no workspace gets the empty state, not an error", code)
	}
	if ws != 0 || role != "" {
		t.Fatalf("workspace = %d role = %q, want unset", ws, role)
	}
}

func TestDefaultWorkspaceFallbackSkippedForBoundAPIKey(t *testing.T) {
	resolver := &stubResolver{ws: &models.Workspace{ID: 9}, role: models.WorkspaceRoleOwner}

	app := okapi.New()
	var gotWorkspace int
	app.Get("/t", func(c *okapi.Context) error {
		gotWorkspace = c.GetInt(CtxWorkspaceID)
		return c.JSON(http.StatusOK, okapi.M{"ok": true})
	}).Use(stashAPIKey(7, models.ScopeAll), OptionalWorkspaceMiddleware(nil, resolver))

	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/t", nil))

	if resolver.called != 0 {
		t.Fatal("a workspace-bound API key is its own workspace; the fallback must not run")
	}
	if gotWorkspace != 7 {
		t.Fatalf("workspace = %d, want the key's binding (7)", gotWorkspace)
	}
}
