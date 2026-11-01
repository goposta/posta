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

	cronpkg "github.com/goposta/posta/internal/cron"
	"github.com/goposta/posta/internal/dto"
	"github.com/goposta/posta/internal/handlers"
	"github.com/goposta/posta/internal/models"
	"github.com/jkaninda/okapi"
)

// adminRoutes returns route definitions for admin endpoints.
func (r *Router) adminRoutes() []okapi.RouteDefinition {
	adminGroup := r.v1.Group("/admin", r.mw.jwtAdminAuth.Middleware).WithTagInfo(okapi.GroupTag{
		Name:        tagAdmin,
		Description: descAdmin,
	})
	adminGroup.WithBearerAuth()

	routes := []okapi.RouteDefinition{
		// ==================== Users ====================
		{
			Method:      http.MethodPost,
			Path:        "/users",
			Handler:     okapi.H(r.h.admin.CreateUser),
			Group:       adminGroup,
			Summary:     "Create a new user",
			Description: "Create a new user account (admin only)",
			Request:     &handlers.AdminCreateUserRequest{},
			Options: []okapi.RouteOption{
				okapi.DocResponse(201, &dto.Response[models.User]{}),
				okapi.DocResponse(409, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:   http.MethodGet,
			Path:     "/users",
			Handler:  okapi.H(r.h.admin.ListUsers),
			Group:    adminGroup,
			Summary:  "List all users",
			Request:  &handlers.ListUsersRequest{},
			Response: &dto.PageableResponse[models.User]{},
		},
		{
			Method:      http.MethodPut,
			Path:        "/users/{id:int}",
			Handler:     okapi.H(r.h.admin.UpdateUser),
			Group:       adminGroup,
			Summary:     "Update user",
			Description: "Update a user's name, email, and/or role",
			Request:     &handlers.AdminUpdateUserRequest{},
			Response:    &dto.Response[models.User]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "User ID"),
				okapi.DocResponse(404, &dto.ErrorResponseBody{}),
				okapi.DocResponse(409, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodGet,
			Path:        "/users/{id:int}/metrics",
			Handler:     okapi.H(r.h.admin.UserMetrics),
			Group:       adminGroup,
			Summary:     "Get user metrics",
			Description: "Returns detailed metrics for a specific user",
			Response:    &dto.Response[handlers.UserDetailMetrics]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "User ID"),
				okapi.DocResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:  http.MethodDelete,
			Path:    "/users/{id:int}",
			Handler: okapi.H(r.h.admin.DeleteUser),
			Group:   adminGroup,
			Tags:    []string{tagAdmin},
			Summary: "Delete user",
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "User ID"),
				okapi.DocResponse(204, nil),
				okapi.DocResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodDelete,
			Path:        "/users/{id:int}/force",
			Handler:     okapi.H(r.h.admin.ForceDeleteUser),
			Group:       adminGroup,
			Tags:        []string{tagAdmin},
			Summary:     "Force delete user",
			Description: "Permanently delete a disabled user and all their data. The user must be disabled before force deletion.",
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "User ID"),
				okapi.DocResponse(204, nil),
				okapi.DocResponse(400, &dto.ErrorResponseBody{}),
				okapi.DocResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodPost,
			Path:        "/users/{id:int}/cancel-deletion",
			Handler:     okapi.H(r.h.admin.CancelUserDeletion),
			Group:       adminGroup,
			Summary:     "Cancel user deletion",
			Description: "Cancel a scheduled account deletion and re-enable the user (admin only)",
			Response:    &dto.Response[okapi.M]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "User ID"),
				okapi.DocResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodDelete,
			Path:        "/users/{id:int}/2fa",
			Handler:     okapi.H(r.h.admin.Disable2FA),
			Group:       adminGroup,
			Summary:     "Disable 2FA for user",
			Description: "Disable two-factor authentication for a user (admin only)",
			Response:    &dto.Response[okapi.M]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "User ID"),
				okapi.DocResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodPost,
			Path:        "/users/{id}/revoke-sessions",
			Handler:     okapi.H(r.h.admin.RevokeUserSessions),
			Group:       adminGroup,
			Summary:     "Revoke all user sessions",
			Description: "Revoke all active sessions for a user (admin only)",
			Response:    &dto.Response[okapi.M]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "User ID"),
				okapi.DocResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodGet,
			Path:        "/users/{id:int}/workspaces",
			Handler:     okapi.H(r.h.admin.UserWorkspaces),
			Group:       adminGroup,
			Summary:     "List user workspaces",
			Description: "Returns all workspaces a user belongs to with plan info",
			Response:    &dto.Response[[]handlers.AdminWorkspace]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "User ID"),
				okapi.DocResponse(404, &dto.ErrorResponseBody{}),
			},
		},

		// ==================== User Plans ====================
		{
			Method:      http.MethodPost,
			Path:        "/users/{id:int}/plan",
			Handler:     okapi.H(r.h.plan.AssignToUser),
			Group:       adminGroup,
			Summary:     "Assign plan to user",
			Description: "Assign a usage plan to a user account",
			Request:     &handlers.AssignUserPlanRequest{},
			Response:    &dto.Response[dto.MessageData]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "User ID"),
				okapi.DocResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodGet,
			Path:        "/users/{id:int}/plan",
			Handler:     okapi.H(r.h.plan.GetUserPlan),
			Group:       adminGroup,
			Summary:     "Get user plan",
			Description: "Get the effective plan for a user",
			Response:    &dto.Response[models.Plan]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "User ID"),
				okapi.DocResponse(404, &dto.ErrorResponseBody{}),
			},
		},

		// ==================== Events ====================
		{
			Method:      http.MethodGet,
			Path:        "/events",
			Handler:     okapi.H(r.h.event.List),
			Group:       adminGroup,
			Summary:     "List events",
			Description: "List platform activity and system events with optional category filter",
			Request:     &handlers.ListEventsRequest{},
			Response:    &dto.PageableResponse[models.Event]{},
		},
		{
			Method:      http.MethodGet,
			Path:        "/events/{id:int}",
			Handler:     okapi.H(r.h.event.Get),
			Group:       adminGroup,
			Summary:     "Get event",
			Description: "Returns a single platform event by ID",
			Request:     &handlers.EventRequest{},
			Response:    &dto.Response[models.Event]{},
		},

		// ==================== Metrics & Analytics ====================
		{
			Method:      http.MethodGet,
			Path:        "/metrics",
			Handler:     r.h.admin.Metrics,
			Group:       adminGroup,
			Summary:     "Platform metrics",
			Description: "Returns platform-wide metrics: users, emails, bounces, suppressions, API keys",
			Response:    &dto.Response[handlers.PlatformMetrics]{},
		},
		{
			Method:      http.MethodGet,
			Path:        "/analytics",
			Handler:     okapi.H(r.h.analytics.AdminAnalytics),
			Group:       adminGroup,
			Summary:     "Platform analytics",
			Description: "Returns platform-wide daily email counts and status breakdown",
			Request:     &handlers.AnalyticsRequest{},
			Response:    &dto.Response[handlers.AnalyticsResponse]{},
		},
		{
			Method:      http.MethodGet,
			Path:        "/analytics/dashboard",
			Handler:     okapi.H(r.h.analytics.AdminDashboardAnalytics),
			Group:       adminGroup,
			Summary:     "Platform dashboard analytics",
			Description: "Returns platform-wide delivery rate trends, bounce rate graphs, and latency percentiles",
			Request:     &handlers.DashboardAnalyticsRequest{},
			Response:    &dto.Response[handlers.DashboardAnalyticsResponse]{},
		},
		{
			Method:      http.MethodGet,
			Path:        "/analytics/providers",
			Handler:     okapi.H(r.h.analytics.AdminProviderBreakdown),
			Group:       adminGroup,
			Summary:     "Platform deliverability by provider",
			Description: "Returns platform-wide sent/failed counts and delivery rate grouped by recipient mailbox provider",
			Request:     &handlers.ProviderBreakdownRequest{},
			Response:    &dto.Response[handlers.ProviderBreakdownResponse]{},
		},

		// ==================== Platform Settings ====================
		{
			Method:   http.MethodGet,
			Path:     pathSettings,
			Handler:  r.h.setting.GetSettings,
			Group:    adminGroup,
			Summary:  "Get platform settings",
			Response: &dto.Response[[]models.Setting]{},
		},
		{
			Method:   http.MethodPut,
			Path:     pathSettings,
			Handler:  okapi.H(r.h.setting.UpdateSettings),
			Group:    adminGroup,
			Summary:  "Update platform settings",
			Request:  &handlers.UpdateSettingsRequest{},
			Response: &dto.Response[[]models.Setting]{},
		},

		// ==================== Updates ====================
		{
			Method:      http.MethodGet,
			Path:        "/update",
			Handler:     r.h.update.GetUpdate,
			Group:       adminGroup,
			Summary:     "Get update status",
			Description: "Returns the cached result of the daily release check for the running build",
			Response:    &dto.Response[handlers.UpdateInfo]{},
		},
		{
			Method:      http.MethodPost,
			Path:        "/update/dismiss",
			Handler:     okapi.H(r.h.update.DismissUpdate),
			Group:       adminGroup,
			Summary:     "Dismiss the update notice for a specific version",
			Description: "Silences the notice until a newer version is released",
			Request:     &handlers.DismissUpdateRequest{},
			Response:    &dto.Response[dto.MessageData]{},
		},

		// ==================== Announcements ====================
		{
			Method:      http.MethodGet,
			Path:        pathAnnouncements,
			Handler:     okapi.H(r.h.announcement.List),
			Group:       adminGroup,
			Summary:     "List announcements",
			Description: "Platform notices that have been broadcast, newest first, with how many inboxes each reached.",
			Request:     &handlers.ListAnnouncementsRequest{},
			Response:    &dto.PageableResponse[models.Announcement]{},
		},
		{
			Method:      http.MethodPost,
			Path:        pathAnnouncements,
			Handler:     okapi.H(r.h.announcement.Create),
			Group:       adminGroup,
			Summary:     "Broadcast an announcement",
			Description: "Delivers a notice to every active user's in-app inbox immediately. There is no draft state; use retract if it went out wrong.",
			Request:     &handlers.CreateAnnouncementRequest{},
			Response:    &dto.Response[models.Announcement]{},
			Options: []okapi.RouteOption{
				okapi.DocResponse(400, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodDelete,
			Path:        pathAnnouncementByID,
			Handler:     okapi.H(r.h.announcement.Retract),
			Group:       adminGroup,
			Summary:     "Retract an announcement",
			Description: "Deletes the announcement and every delivery it made, removing it from users' inboxes.",
			Response:    &dto.Response[dto.MessageData]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Announcement ID"),
				okapi.DocResponse(404, &dto.ErrorResponseBody{}),
			},
		},

		// ==================== Domains ====================
		{
			Method:      http.MethodGet,
			Path:        pathDomains,
			Handler:     okapi.H(r.h.adminDomain.List),
			Group:       adminGroup,
			Summary:     "List domains across every workspace",
			Description: "Platform-wide domain list, filterable by name, ownership verification, and workspace.",
			Request:     &handlers.AdminListDomainsRequest{},
			Response:    &dto.PageableResponse[handlers.AdminDomainRow]{},
		},
		{
			Method:      http.MethodGet,
			Path:        pathDomainByID,
			Handler:     okapi.H(r.h.adminDomain.Get),
			Group:       adminGroup,
			Summary:     "Get a domain",
			Description: "Returns the domain with its owner, the DNS records still required, and any workspace already holding the name verified.",
			Response:    &dto.Response[handlers.AdminDomainDetail]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Domain ID"),
				okapi.DocResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodPost,
			Path:        pathDomainByID + "/verify",
			Handler:     okapi.H(r.h.adminDomain.Verify),
			Group:       adminGroup,
			Summary:     "Run DNS verification for a domain",
			Description: "Performs the same DNS checks the owning workspace would, on their behalf.",
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Domain ID"),
				okapi.DocResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:  http.MethodPut,
			Path:    pathDomainByID + "/verification",
			Handler: okapi.H(r.h.adminDomain.SetVerification),
			Group:   adminGroup,
			Summary: "Set domain ownership manually",
			Description: "Marks ownership verified, or revokes it, without a DNS check. Requires a reason when granting, " +
				"and is refused when another workspace already holds the name verified.",
			Request:  &handlers.AdminSetDomainVerificationRequest{},
			Response: &dto.Response[models.Domain]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Domain ID"),
				okapi.DocResponse(400, &dto.ErrorResponseBody{}),
				okapi.DocResponse(409, &dto.ErrorResponseBody{}),
			},
		},

		// ==================== Workspaces ====================
		{
			Method:      http.MethodGet,
			Path:        pathWorkspaces,
			Handler:     okapi.H(r.h.adminWorkspace.List),
			Group:       adminGroup,
			Summary:     "List all workspaces",
			Description: "Platform-wide workspace list with owner, plan and member count, filterable by name/slug, owner, and plan.",
			Request:     &handlers.AdminListWorkspacesRequest{},
			Response:    &dto.PageableResponse[handlers.AdminWorkspaceRow]{},
		},
		{
			Method:      http.MethodGet,
			Path:        pathWorkspaceByID,
			Handler:     okapi.H(r.h.adminWorkspace.Get),
			Group:       adminGroup,
			Summary:     "Get a workspace",
			Description: "Returns the workspace with its members, pending invitation count, and resource usage.",
			Response:    &dto.Response[handlers.AdminWorkspaceDetail]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Workspace ID"),
				okapi.DocResponse(404, &dto.ErrorResponseBody{}),
			},
		},

		// ==================== Plans ====================
		{
			Method:      http.MethodPost,
			Path:        "/plans",
			Handler:     okapi.H(r.h.plan.Create),
			Group:       adminGroup,
			Summary:     "Create plan",
			Description: "Create a new usage plan/package",
			Request:     &handlers.CreatePlanRequest{},
			Options: []okapi.RouteOption{
				okapi.DocResponse(201, &dto.Response[models.Plan]{}),
				okapi.DocResponse(409, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:   http.MethodGet,
			Path:     "/plans",
			Handler:  okapi.H(r.h.plan.List),
			Group:    adminGroup,
			Summary:  "List plans",
			Request:  &handlers.ListPlansRequest{},
			Response: &dto.PageableResponse[models.Plan]{},
		},
		{
			Method:   http.MethodGet,
			Path:     pathPlanByID,
			Handler:  okapi.H(r.h.plan.Get),
			Group:    adminGroup,
			Summary:  "Get plan",
			Response: &dto.Response[models.Plan]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Plan ID"),
				okapi.DocResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodPut,
			Path:        pathPlanByID,
			Handler:     okapi.H(r.h.plan.Update),
			Group:       adminGroup,
			Summary:     "Update plan",
			Description: "Update a plan's configuration and limits",
			Request:     &handlers.UpdatePlanRequest{},
			Response:    &dto.Response[models.Plan]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Plan ID"),
				okapi.DocResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodDelete,
			Path:        pathPlanByID,
			Handler:     okapi.H(r.h.plan.Delete),
			Group:       adminGroup,
			Summary:     "Delete plan",
			Description: "Delete a plan. Use ?force=true to delete a plan assigned to workspaces.",
			Request:     &handlers.DeletePlanRequest{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Plan ID"),
				okapi.DocResponse(204, nil),
				okapi.DocResponse(404, &dto.ErrorResponseBody{}),
				okapi.DocResponse(409, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodPatch,
			Path:        "/plans/{id:int}/default",
			Handler:     okapi.H(r.h.plan.SetDefault),
			Group:       adminGroup,
			Summary:     "Set plan as default",
			Description: "Set this plan as the default plan, unsetting any previous default",
			Response:    &dto.Response[models.Plan]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Plan ID"),
				okapi.DocResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodPost,
			Path:        "/workspaces/{id:int}/plan",
			Handler:     okapi.H(r.h.plan.AssignToWorkspace),
			Group:       adminGroup,
			Summary:     "Assign plan to workspace",
			Description: "Assign a usage plan to a workspace",
			Request:     &handlers.AssignWorkspacePlanRequest{},
			Response:    &dto.Response[dto.MessageData]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Workspace ID"),
				okapi.DocResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodGet,
			Path:        "/workspaces/{id:int}/plan",
			Handler:     okapi.H(r.h.plan.GetWorkspacePlan),
			Group:       adminGroup,
			Summary:     "Get workspace plan",
			Description: "Get the effective plan for a workspace",
			Response:    &dto.Response[models.Plan]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Workspace ID"),
				okapi.DocResponse(404, &dto.ErrorResponseBody{}),
			},
		},

		// ==================== Shared SMTP Servers ====================
		{
			Method:      http.MethodPost,
			Path:        "/servers",
			Handler:     okapi.H(r.h.server.Create),
			Group:       adminGroup,
			Summary:     "Create shared SMTP server",
			Description: "Add a new shared SMTP server to the pool",
			Request:     &handlers.CreateServerRequest{},
			Options: []okapi.RouteOption{
				okapi.DocResponse(201, &dto.Response[models.Server]{}),
			},
		},
		{
			Method:   http.MethodGet,
			Path:     "/servers",
			Handler:  okapi.H(r.h.server.List),
			Group:    adminGroup,
			Summary:  "List shared SMTP servers",
			Request:  &handlers.ListServersRequest{},
			Response: &dto.PageableResponse[models.Server]{},
		},
		{
			Method:   http.MethodGet,
			Path:     pathServerByID,
			Handler:  okapi.H(r.h.server.Get),
			Group:    adminGroup,
			Summary:  "Get shared SMTP server",
			Response: &dto.Response[models.Server]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Server ID"),
				okapi.DocResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:   http.MethodPut,
			Path:     pathServerByID,
			Handler:  okapi.H(r.h.server.Update),
			Group:    adminGroup,
			Summary:  "Update shared SMTP server",
			Request:  &handlers.UpdateServerRequest{},
			Response: &dto.Response[models.Server]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Server ID"),
				okapi.DocResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:  http.MethodDelete,
			Path:    pathServerByID,
			Handler: okapi.H(r.h.server.Delete),
			Group:   adminGroup,
			Tags:    []string{tagAdmin},
			Summary: "Delete shared SMTP server",
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Server ID"),
				okapi.DocResponse(204, nil),
				okapi.DocResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:   http.MethodPost,
			Path:     "/servers/{id:int}/enable",
			Handler:  okapi.H(r.h.server.Enable),
			Group:    adminGroup,
			Summary:  "Enable shared SMTP server",
			Response: &dto.Response[models.Server]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Server ID"),
				okapi.DocResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:   http.MethodPost,
			Path:     "/servers/{id:int}/disable",
			Handler:  okapi.H(r.h.server.Disable),
			Group:    adminGroup,
			Summary:  "Disable shared SMTP server",
			Response: &dto.Response[models.Server]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Server ID"),
				okapi.DocResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:   http.MethodPost,
			Path:     "/servers/{id:int}/test",
			Handler:  okapi.H(r.h.server.Test),
			Group:    adminGroup,
			Summary:  "Test shared SMTP server connection",
			Response: &dto.Response[dto.MessageData]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Server ID"),
				okapi.DocResponse(404, &dto.ErrorResponseBody{}),
			},
		},
	}

	// Cron jobs (only if cron manager is configured)
	if r.h.cron != nil {
		routes = append(routes, okapi.RouteDefinition{
			Method:      http.MethodGet,
			Path:        "/jobs",
			Handler:     r.h.cron.List,
			Group:       adminGroup,
			Summary:     "List scheduled jobs",
			Description: "Returns all registered cron jobs with their schedule and last execution status.",
			Response:    &dto.Response[[]cronpkg.JobStatus]{},
		})
	}

	return routes
}

// adminSSERoutes returns route definitions for admin SSE (Server-Sent Events) endpoints.
func (r *Router) adminSSERoutes() []okapi.RouteDefinition {
	adminSSE := r.v1.Group("/admin", r.mw.jwtAdminAuth.Middleware).WithTagInfo(okapi.GroupTag{
		Name:        tagAdmin,
		Description: descAdmin,
	})

	return []okapi.RouteDefinition{
		{
			Method:      http.MethodGet,
			Path:        "/events/stream",
			Handler:     r.h.event.Stream,
			Group:       adminSSE,
			Summary:     "Stream events (SSE)",
			Description: "Real-time event stream via Server-Sent Events. Authenticated by the session cookie the browser sends on the EventSource handshake.",
			Options:     []okapi.RouteOption{okapi.DocHide()},
		},
		{
			Method:      http.MethodGet,
			Path:        "/metrics/stream",
			Handler:     r.h.admin.MetricsStream,
			Group:       adminSSE,
			Summary:     "Stream platform metrics (SSE)",
			Description: "Real-time worker status and system runtime stats via Server-Sent Events. Emits worker.status and system.status events. Authenticated by the session cookie.",
			Options:     []okapi.RouteOption{okapi.DocHide()},
		},
	}
}
