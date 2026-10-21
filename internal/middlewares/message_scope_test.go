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

// GDPR erasure is destructive tenant administration. A workspace-bound key
// resolves as owner, so without the admin scope a write-only key could erase
// every contact in the workspace.
func TestGDPRRoutesRequireAdminScope(t *testing.T) {
	for _, path := range []string{
		"/api/v1/workspaces/current/gdpr/delete-contacts",
		"/api/v1/workspaces/current/gdpr/delete-email-logs",
		"/api/v1/workspaces/current/gdpr/delete-messages",
	} {
		if got := workspaceScopeFor(http.MethodPost, path); got != models.ScopeAdmin {
			t.Fatalf("workspaceScopeFor(POST %s) = %s, want admin", path, got)
		}
	}
	key := &models.APIKey{Scopes: []string{models.ScopeRead, models.ScopeWrite}}
	if key.HasScope(workspaceScopeFor(http.MethodPost, "/api/v1/workspaces/current/gdpr/delete-contacts")) {
		t.Fatal("a read/write key must not reach GDPR erasure")
	}
}
