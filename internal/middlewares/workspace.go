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
	"strconv"

	"github.com/jkaninda/okapi"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/storage/repositories"
)

// OptionalWorkspaceMiddleware reads the X-Posta-Workspace-Id header if present.
// If absent, the request operates in personal mode (workspace_id stays 0).
// If present, validates membership and sets workspace_id + workspace_role.
func OptionalWorkspaceMiddleware(workspaceRepo *repositories.WorkspaceRepository) okapi.Middleware {
	return func(c *okapi.Context) error {
		wsHeader := c.Header("X-Posta-Workspace-Id")
		if wsHeader == "" {
			// Personal mode — no workspace context
			return c.Next()
		}

		userID := c.GetInt("user_id")
		if userID == 0 {
			return c.AbortUnauthorized("authentication required")
		}

		wsID, err := strconv.Atoi(wsHeader)
		if err != nil || wsID <= 0 {
			return c.AbortBadRequest("invalid X-Posta-Workspace-Id")
		}

		member, err := workspaceRepo.FindMember(uint(wsID), uint(userID))
		if err != nil {
			return c.AbortForbidden("you are not a member of this workspace")
		}

		c.Set("workspace_id", wsID)
		c.Set("workspace_role", string(member.Role))

		return c.Next()
	}
}

// RequireWorkspaceMiddleware is like OptionalWorkspaceMiddleware but requires
// the header to be present (for workspace-only endpoints like member management).
func RequireWorkspaceMiddleware(workspaceRepo *repositories.WorkspaceRepository) okapi.Middleware {
	return func(c *okapi.Context) error {
		wsHeader := c.Header("X-Posta-Workspace-Id")
		if wsHeader == "" {
			return c.AbortBadRequest("X-Posta-Workspace-Id header is required")
		}

		userID := c.GetInt("user_id")
		if userID == 0 {
			return c.AbortUnauthorized("authentication required")
		}

		wsID, err := strconv.Atoi(wsHeader)
		if err != nil || wsID <= 0 {
			return c.AbortBadRequest("invalid X-Posta-Workspace-Id")
		}

		member, err := workspaceRepo.FindMember(uint(wsID), uint(userID))
		if err != nil {
			return c.AbortForbidden("you are not a member of this workspace")
		}

		c.Set("workspace_id", wsID)
		c.Set("workspace_role", string(member.Role))

		return c.Next()
	}
}

// RequireWorkspaceRole returns a middleware that enforces a minimum workspace role.
func RequireWorkspaceRole(minRole models.WorkspaceRole) okapi.Middleware {
	return func(c *okapi.Context) error {
		roleStr := c.GetString("workspace_role")
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
