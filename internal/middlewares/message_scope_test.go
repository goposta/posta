// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package middlewares

import (
	"net/http"
	"testing"

	"github.com/goposta/posta/internal/models"
)

func TestWorkspaceScopeForMessageRoutes(t *testing.T) {
	cases := []struct {
		method string
		path   string
		want   string
	}{
		{http.MethodGet, "/api/v1/workspaces/current/messages", models.ScopeRead},
		{http.MethodGet, "/api/v1/workspaces/current/messages/abc", models.ScopeRead},
		{http.MethodPost, "/api/v1/workspaces/current/messages/abc/reply", models.ScopeWrite},
		{http.MethodDelete, "/api/v1/workspaces/current/messages/abc", models.ScopeWrite},

		{http.MethodGet, "/api/v1/workspaces/current/forms", models.ScopeAdmin},
		{http.MethodPost, "/api/v1/workspaces/current/forms", models.ScopeAdmin},
		{http.MethodPut, "/api/v1/workspaces/current/forms/1", models.ScopeAdmin},
		{http.MethodPost, "/api/v1/workspaces/current/forms/1/rotate-key", models.ScopeAdmin},

		{http.MethodGet, "/api/v1/workspaces/current/message-filters", models.ScopeAdmin},
		{http.MethodPost, "/api/v1/workspaces/current/message-filters", models.ScopeAdmin},
	}

	for _, tc := range cases {
		if got := workspaceScopeFor(tc.method, tc.path); got != tc.want {
			t.Fatalf("workspaceScopeFor(%s %s) = %s, want %s", tc.method, tc.path, got, tc.want)
		}
	}
}

func TestSendScopeCannotReachMessages(t *testing.T) {
	key := &models.APIKey{Scopes: []string{models.ScopeSend}}
	for _, scope := range []string{models.ScopeRead, models.ScopeWrite, models.ScopeAdmin} {
		if key.HasScope(scope) {
			t.Fatalf("a send-only key must not carry %s", scope)
		}
	}
}

func TestReadScopeCannotReachFormConfig(t *testing.T) {
	key := &models.APIKey{Scopes: []string{models.ScopeRead}}
	required := workspaceScopeFor(http.MethodGet, "/api/v1/workspaces/current/forms")
	if key.HasScope(required) {
		t.Fatal("a read-only key must not be able to enumerate form configuration")
	}
}
