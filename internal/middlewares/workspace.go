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

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
)

// WorkspaceHeader names the header carrying the active workspace. Browser-direct
// requests (EventSource, download links) cannot set headers, so the
// query-tolerant middleware also accepts `?workspace_id=`.
const WorkspaceHeader = "X-Posta-Workspace-Id"

// resolveWorkspace establishes the request's workspace and the caller's role in
// it, given the raw workspace id the caller asked for ("" when unspecified).
//
// Three callers, three sources of authority:
//
//   - A workspace-bound API key IS the workspace. It gets owner-equivalent
//     access to it and cannot reach any other workspace, whatever it asks for.
//   - An account-wide API key, and any JWT session, must prove membership of the
//     workspace they name, and inherit that membership's role.
//   - When nobody names a workspace and `required` is false, we fall back to the
//     user's personal workspace.
func resolveWorkspace(c *okapi.Context, workspaceRepo *repositories.WorkspaceRepository, userRepo *repositories.UserRepository, raw string, required bool) error {
	userID := c.GetInt(CtxUserID)
	if userID == 0 {
		return c.AbortUnauthorized("authentication required")
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

	// Nobody named a workspace: fall back to the caller's personal one. An
	// unmigrated user has none — that is the legacy personal mode, not an error.
	if userRepo != nil {
		if personalID, err := userRepo.PersonalWorkspaceID(uint(userID)); err == nil && personalID != nil {
			c.Set(CtxWorkspaceID, int(*personalID))
			c.Set(CtxWorkspaceRole, string(models.WorkspaceRoleOwner))
		}
	}
	return c.Next()
}

// RequireWorkspaceMiddleware demands an explicit workspace, except for a
// workspace-bound API key, whose binding names it.
func RequireWorkspaceMiddleware(workspaceRepo *repositories.WorkspaceRepository, userRepo *repositories.UserRepository) okapi.Middleware {
	return func(c *okapi.Context) error {
		return resolveWorkspace(c, workspaceRepo, userRepo, c.Header(WorkspaceHeader), true)
	}
}

// OptionalWorkspaceMiddleware resolves the workspace when named, and otherwise
// falls back to the caller's personal workspace.
func OptionalWorkspaceMiddleware(workspaceRepo *repositories.WorkspaceRepository, userRepo *repositories.UserRepository) okapi.Middleware {
	return func(c *okapi.Context) error {
		return resolveWorkspace(c, workspaceRepo, userRepo, c.Header(WorkspaceHeader), false)
	}
}

// WorkspaceFromQueryOrHeader is OptionalWorkspaceMiddleware for browser-direct
// requests, which can pass the workspace only as a query parameter.
func WorkspaceFromQueryOrHeader(workspaceRepo *repositories.WorkspaceRepository, userRepo *repositories.UserRepository) okapi.Middleware {
	return func(c *okapi.Context) error {
		raw := c.Header(WorkspaceHeader)
		if raw == "" {
			raw = c.Query("workspace_id")
		}
		return resolveWorkspace(c, workspaceRepo, userRepo, raw, false)
	}
}

// RequireWorkspaceRole enforces a minimum workspace role. A workspace-bound API
// key resolves as owner, so it clears every tier.
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
