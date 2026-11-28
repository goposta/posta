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

package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/goposta/posta/internal/models"
	"github.com/jkaninda/okapi"
	"gorm.io/gorm"
)

var errQuotaReached = errors.New("quota reached")

// Fails every check, so a request that gets past the scope check stops here without touching the database.
type exhaustedQuota struct{}

func (exhaustedQuota) CheckQuota(*gorm.DB, uint, *uint, string) error { return errQuotaReached }
func (exhaustedQuota) CheckWorkspaceQuota(*gorm.DB, uint) error       { return errQuotaReached }

func TestCreateAPIKeyAdminScopeNeedsAdminRole(t *testing.T) {
	h := &APIKeyHandler{quota: exhaustedQuota{}}
	create := func(role models.WorkspaceRole, scopes ...string) (int, string) {
		rec := httptest.NewRecorder()
		c := okapi.NewContext(okapi.New(), rec, httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/current/api-keys", nil))
		c.Set("user_id", 3)
		c.Set("workspace_id", 5)
		c.Set("workspace_role", string(role))
		req := &CreateAPIKeyRequest{}
		req.Body.Name = "ci"
		req.Body.Scopes = scopes
		_ = h.Create(c, req)
		return rec.Code, rec.Body.String()
	}

	tests := []struct {
		role    models.WorkspaceRole
		scopes  []string
		refused bool
	}{
		{models.WorkspaceRoleEditor, []string{models.ScopeAll}, true},
		{models.WorkspaceRoleEditor, []string{models.ScopeAdmin}, true},
		{models.WorkspaceRoleEditor, []string{models.ScopeSend, models.ScopeAdmin}, true},
		{models.WorkspaceRoleEditor, []string{models.ScopeSend, models.ScopeWrite}, false},
		{models.WorkspaceRoleAdmin, []string{models.ScopeAll}, false},
		{models.WorkspaceRoleOwner, []string{models.ScopeAdmin}, false},
	}
	for _, tc := range tests {
		t.Run(string(tc.role)+"/"+strings.Join(tc.scopes, ","), func(t *testing.T) {
			code, body := create(tc.role, tc.scopes...)
			if code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403 (scope refusal or quota stop); body %s", code, body)
			}
			refused := strings.Contains(body, "admin scope")
			if refused != tc.refused {
				t.Fatalf("scope refused = %v, want %v; body %s", refused, tc.refused, body)
			}
			if !tc.refused && !strings.Contains(body, "quota") {
				t.Fatalf("want the request to reach the quota check; body %s", body)
			}
		})
	}

	if code, _ := create(models.WorkspaceRoleViewer, models.ScopeSend); code != http.StatusForbidden {
		t.Fatalf("viewer status = %d, want 403", code)
	}
}
