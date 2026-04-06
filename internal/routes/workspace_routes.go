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

	"github.com/goposta/posta/internal/dto"
	"github.com/goposta/posta/internal/handlers"
	"github.com/goposta/posta/internal/models"
	"github.com/jkaninda/okapi"
)

// workspaceRoutes returns route definitions for workspace management.
func (r *Router) workspaceRoutes() []okapi.RouteDefinition {
	// Routes that don't require workspace context (user-level)
	userGroup := r.v1.Group("/workspaces", r.mw.jwtAuth.Middleware).WithTags([]string{"Workspaces"})
	userGroup.WithBearerAuth()

	routes := make([]okapi.RouteDefinition, 0, 20)
	routes = append(routes, []okapi.RouteDefinition{
		{
			Method:      http.MethodPost,
			Path:        "",
			Handler:     okapi.H(r.h.workspace.Create),
			Group:       userGroup,
			Summary:     "Create workspace",
			Description: "Create a new workspace. The creator becomes the owner.",
			Request:     &handlers.CreateWorkspaceRequest{},
			Options: []okapi.RouteOption{
				okapi.DocResponse(201, &dto.Response[handlers.WorkspaceResponse]{}),
				okapi.DocErrorResponse(409, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodGet,
			Path:        "",
			Handler:     r.h.workspace.List,
			Group:       userGroup,
			Summary:     "List workspaces",
			Description: "List all workspaces the current user is a member of",
			Response:    &dto.Response[[]handlers.WorkspaceResponse]{},
		},
	}...)

	// Workspace-scoped routes (require workspace context via middleware)
	wsGroup := r.v1.Group("/workspaces/current", r.mw.jwtAuth.Middleware, r.mw.workspace).WithTags([]string{"Workspaces"})
	wsGroup.WithBearerAuth()

	routes = append(routes, []okapi.RouteDefinition{
		{
			Method:   http.MethodGet,
			Path:     "",
			Handler:  r.h.workspace.Get,
			Group:    wsGroup,
			Summary:  "Get current workspace",
			Response: &dto.Response[handlers.WorkspaceResponse]{},
			Options:  []okapi.RouteOption{workspaceHeaderRequired},
		},
		{
			Method:      http.MethodPut,
			Path:        "",
			Handler:     okapi.H(r.h.workspace.Update),
			Group:       wsGroup,
			Summary:     "Update workspace",
			Description: "Update workspace name and description (admin/owner only)",
			Request:     &handlers.UpdateWorkspaceRequest{},
			Response:    &dto.Response[handlers.WorkspaceResponse]{},
			Options:     []okapi.RouteOption{workspaceHeaderRequired},
		},
		{
			Method:      http.MethodDelete,
			Path:        "",
			Handler:     r.h.workspace.Delete,
			Group:       wsGroup,
			Summary:     "Delete workspace",
			Description: "Delete the workspace (owner only)",
			Options: []okapi.RouteOption{
				workspaceHeaderRequired,
				okapi.DocResponse(204, nil),
			},
		},

		// Members
		{
			Method:   http.MethodGet,
			Path:     "/members",
			Handler:  r.h.workspace.ListMembers,
			Group:    wsGroup,
			Summary:  "List workspace members",
			Response: &dto.Response[[]handlers.WorkspaceMemberResponse]{},
			Options:  []okapi.RouteOption{workspaceHeaderRequired},
		},
		{
			Method:   http.MethodPut,
			Path:     "/members/{member_id:int}",
			Handler:  okapi.H(r.h.workspace.UpdateMemberRole),
			Group:    wsGroup,
			Summary:  "Update member role",
			Request:  &handlers.UpdateMemberRoleRequest{},
			Response: &dto.Response[dto.MessageData]{},
			Options: []okapi.RouteOption{
				workspaceHeaderRequired,
				okapi.DocPathParam("member_id", "integer", "User ID of the member"),
			},
		},
		{
			Method:      http.MethodDelete,
			Path:        "/members/{member_id:int}",
			Handler:     okapi.H(r.h.workspace.RemoveMember),
			Group:       wsGroup,
			Summary:     "Remove member",
			Description: "Remove a member from the workspace (admin/owner only)",
			Options: []okapi.RouteOption{
				workspaceHeaderRequired,
				okapi.DocPathParam("member_id", "integer", "User ID of the member"),
				okapi.DocResponse(204, nil),
			},
		},

		// Invitations (workspace-scoped)
		{
			Method:      http.MethodPost,
			Path:        "/invitations",
			Handler:     okapi.H(r.h.workspace.InviteMember),
			Group:       wsGroup,
			Summary:     "Invite member",
			Description: "Invite a user to the workspace via email",
			Request:     &handlers.InviteMemberRequest{},
			Options: []okapi.RouteOption{
				workspaceHeaderRequired,
				okapi.DocResponse(201, &dto.Response[handlers.InvitationResponse]{}),
			},
		},
		{
			Method:   http.MethodGet,
			Path:     "/invitations",
			Handler:  r.h.workspace.ListInvitations,
			Group:    wsGroup,
			Summary:  "List pending invitations",
			Response: &dto.Response[[]handlers.InvitationResponse]{},
			Options:  []okapi.RouteOption{workspaceHeaderRequired},
		},
		{
			Method:  http.MethodDelete,
			Path:    "/invitations/{invitation_id:int}",
			Handler: okapi.H(r.h.workspace.DeleteInvitation),
			Group:   wsGroup,
			Summary: "Cancel invitation",
			Options: []okapi.RouteOption{
				workspaceHeaderRequired,
				okapi.DocPathParam("invitation_id", "integer", "Invitation ID"),
				okapi.DocResponse(204, nil),
			},
		},

		// Plan
		{
			Method:      http.MethodGet,
			Path:        "/plan",
			Handler:     r.h.workspace.GetPlan,
			Group:       wsGroup,
			Summary:     "Get workspace plan",
			Description: "Get the effective plan and limits for the current workspace",
			Response:    &dto.Response[models.Plan]{},
			Options:     []okapi.RouteOption{workspaceHeaderRequired},
		},

		// Data Transfer
		{
			Method:      http.MethodPost,
			Path:        "/transfer",
			Handler:     okapi.H(r.h.workspace.TransferData),
			Group:       wsGroup,
			Summary:     "Transfer personal data to workspace",
			Description: "Move the current user's personal resources into this workspace",
			Request:     &handlers.TransferDataRequest{},
			Response:    &dto.Response[handlers.TransferResponse]{},
			Options:     []okapi.RouteOption{workspaceHeaderRequired},
		},

		// Data Export/Import
		{
			Method:      http.MethodGet,
			Path:        "/data/export",
			Handler:     r.h.workspaceData.Export,
			Group:       wsGroup,
			Summary:     "Export workspace data",
			Description: "Export all workspace data including settings, templates, stylesheets, languages, contacts, contact lists, webhooks, SMTP servers, domains, subscribers, subscriber lists, and suppressions as JSON",
			Response:    &dto.Response[handlers.WorkspaceDataExport]{},
			Options:     []okapi.RouteOption{workspaceHeaderRequired},
		},
		{
			Method:      http.MethodPost,
			Path:        "/data/import",
			Handler:     okapi.H(r.h.workspaceData.Import),
			Group:       wsGroup,
			Summary:     "Import workspace data",
			Description: "Import workspace data from a previously exported JSON payload. Duplicates are skipped. SMTP servers are imported as disabled (passwords excluded). Domains require re-verification.",
			Request:     &handlers.ImportWorkspaceDataRequest{},
			Response:    &dto.Response[any]{},
			Options:     []okapi.RouteOption{workspaceHeaderRequired},
		},
	}...)

	// User-level invitation actions (no workspace context needed)
	invGroup := r.v1.Group("/invitations", r.mw.jwtAuth.Middleware).WithTags([]string{"Workspaces"})
	invGroup.WithBearerAuth()

	routes = append(routes, []okapi.RouteDefinition{
		{
			Method:      http.MethodGet,
			Path:        "",
			Handler:     r.h.workspace.MyInvitations,
			Group:       invGroup,
			Summary:     "My pending invitations",
			Description: "List all pending workspace invitations for the current user",
			Response:    &dto.Response[[]handlers.InvitationResponse]{},
		},
		{
			Method:      http.MethodPost,
			Path:        "/accept",
			Handler:     okapi.H(r.h.workspace.AcceptInvitation),
			Group:       invGroup,
			Summary:     "Accept invitation",
			Description: "Accept a workspace invitation using the invitation token",
			Request:     &handlers.AcceptInvitationRequest{},
			Response:    &dto.Response[okapi.M]{},
		},
		{
			Method:      http.MethodPost,
			Path:        "/decline",
			Handler:     okapi.H(r.h.workspace.DeclineInvitation),
			Group:       invGroup,
			Summary:     "Decline invitation by token",
			Description: "Decline a workspace invitation using the invitation token",
			Request:     &handlers.DeclineInvitationRequest{},
			Response:    &dto.Response[okapi.M]{},
		},
		{
			Method:      http.MethodPost,
			Path:        "/{id:int}/accept",
			Handler:     okapi.H(r.h.workspace.AcceptInvitationByID),
			Group:       invGroup,
			Summary:     "Accept invitation by ID",
			Description: "Accept a workspace invitation by its ID",
			Response:    &dto.Response[okapi.M]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Invitation ID"),
			},
		},
		{
			Method:      http.MethodPost,
			Path:        "/{id:int}/decline",
			Handler:     okapi.H(r.h.workspace.DeclineInvitationByID),
			Group:       invGroup,
			Summary:     "Decline invitation by ID",
			Description: "Decline a workspace invitation by its ID",
			Response:    &dto.Response[okapi.M]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Invitation ID"),
			},
		},
	}...)

	return routes
}

// Ignore unused imports for linting
var _ = models.WorkspaceRoleOwner
