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
	"strconv"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
)

// Browser-direct requests cannot set headers, so the query-tolerant middleware also accepts ?workspace_id=.
const WorkspaceHeader = "X-Posta-Workspace-Id"

// Swappable for tests: the role it returns is all that stops a viewer getting write access on header-less requests.
type DefaultWorkspaceResolver interface {
	DefaultWorkspace(userID uint) (*models.Workspace, models.WorkspaceRole, error)
}

// A workspace-bound key IS the workspace; other callers must prove membership and inherit its role.
func resolveWorkspace(c *okapi.Context, workspaceRepo *repositories.WorkspaceRepository, userRepo DefaultWorkspaceResolver, raw string, required bool) error {
	userID := c.GetInt(CtxUserID)
	if userID == 0 {
		return c.AbortUnauthorized("authentication required")
	}

	// Checked before resolving, so an under-scoped key learns nothing about whether the named workspace exists.
	if err := requireWorkspaceScope(c); err != nil {
		return err
	}

	if bound := APIKeyWorkspaceID(c); bound != nil {
		if raw != "" {
			wsID, err := strconv.Atoi(raw)
			if err != nil || wsID <= 0 {
				return c.AbortBadRequest("invalid workspace id")
			}
			if uint(wsID) != *bound {
				return c.AbortForbidden("API key is not bound to this workspace")
			}
		}
		c.Set(CtxWorkspaceID, int(*bound))
		c.Set(CtxWorkspaceRole, string(models.WorkspaceRoleOwner))
		return c.Next()
	}

	if raw != "" {
		wsID, err := strconv.Atoi(raw)
		if err != nil || wsID <= 0 {
			return c.AbortBadRequest("invalid workspace id")
		}
		member, err := workspaceRepo.FindMember(uint(wsID), uint(userID))
		if err != nil {
			return c.AbortForbidden("you are not a member of this workspace")
		}
		c.Set(CtxWorkspaceID, wsID)
		c.Set(CtxWorkspaceRole, string(member.Role))
		return c.Next()
	}

	if required {
		return c.AbortBadRequest(WorkspaceHeader + " header is required")
	}

	// The role comes from the membership, never a constant: the default workspace may be one the caller only views.
	if userRepo != nil {
		if ws, role, err := userRepo.DefaultWorkspace(uint(userID)); err == nil && ws != nil {
			c.Set(CtxWorkspaceID, int(ws.ID))
			c.Set(CtxWorkspaceRole, string(role))
		}
	}
	return c.Next()
}

// A workspace-bound API key needs no explicit workspace: its binding names it.
func RequireWorkspaceMiddleware(workspaceRepo *repositories.WorkspaceRepository, userRepo DefaultWorkspaceResolver) okapi.Middleware {
	return func(c *okapi.Context) error {
		return resolveWorkspace(c, workspaceRepo, userRepo, c.Header(WorkspaceHeader), true)
	}
}

func OptionalWorkspaceMiddleware(workspaceRepo *repositories.WorkspaceRepository, userRepo DefaultWorkspaceResolver) okapi.Middleware {
	return func(c *okapi.Context) error {
		return resolveWorkspace(c, workspaceRepo, userRepo, c.Header(WorkspaceHeader), false)
	}
}

// Browser-direct requests can pass the workspace only as a query parameter.
func WorkspaceFromQueryOrHeader(workspaceRepo *repositories.WorkspaceRepository, userRepo DefaultWorkspaceResolver) okapi.Middleware {
	return func(c *okapi.Context) error {
		raw := c.Header(WorkspaceHeader)
		if raw == "" {
			raw = c.Query("workspace_id")
		}
		return resolveWorkspace(c, workspaceRepo, userRepo, raw, false)
	}
}

// A workspace-bound API key resolves as owner, so it clears every tier.
func RequireWorkspaceRole(minRole models.WorkspaceRole) okapi.Middleware {
	return func(c *okapi.Context) error {
		roleStr := c.GetString(CtxWorkspaceRole)
		if roleStr == "" {
			return c.AbortForbidden("workspace context required")
		}

		role := models.WorkspaceRole(roleStr)
		allowed := false

		switch minRole {
		case models.WorkspaceRoleViewer:
			allowed = role.CanView()
		case models.WorkspaceRoleEditor:
			allowed = role.CanEdit()
		case models.WorkspaceRoleAdmin:
			allowed = role.CanManageMembers()
		case models.WorkspaceRoleOwner:
			allowed = role.IsOwner()
		}

		if !allowed {
			return c.AbortForbidden("insufficient workspace permissions")
		}

		return c.Next()
	}
}
