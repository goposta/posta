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

	cronpkg "github.com/goposta/posta/internal/cron"
	"github.com/goposta/posta/internal/dto"
	"github.com/goposta/posta/internal/handlers"
	"github.com/goposta/posta/internal/models"
	"github.com/jkaninda/okapi"
)

// adminRoutes returns route definitions for admin endpoints.
func (r *Router) adminRoutes() []okapi.RouteDefinition {
	adminGroup := r.v1.Group("/admin", r.mw.jwtAdminAuth.Middleware).WithTags([]string{"Admin"})
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
				okapi.DocErrorResponse(409, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:   http.MethodGet,
			Path:     "/users",
			Handler:  okapi.H(r.h.admin.ListUsers),
			Group:    adminGroup,
			Summary:  "List all users",
			Request:  &handlers.ListRequest{},
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
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
				okapi.DocErrorResponse(409, &dto.ErrorResponseBody{}),
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
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:  http.MethodDelete,
			Path:    "/users/{id:int}",
			Handler: okapi.H(r.h.admin.DeleteUser),
			Group:   adminGroup,
			Tags:    []string{"Admin"},
			Summary: "Delete user",
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "User ID"),
				okapi.DocResponse(204, nil),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
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
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
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
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodPost,
			Path:        "/users/{id:int}/revoke-sessions",
			Handler:     okapi.H(r.h.admin.RevokeUserSessions),
			Group:       adminGroup,
			Summary:     "Revoke all user sessions",
			Description: "Revoke all active sessions for a user (admin only)",
			Response:    &dto.Response[okapi.M]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "User ID"),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
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
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
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

		// ==================== Platform Settings ====================
		{
			Method:   http.MethodGet,
			Path:     "/settings",
			Handler:  r.h.setting.GetSettings,
			Group:    adminGroup,
			Summary:  "Get platform settings",
			Response: &dto.Response[[]models.Setting]{},
		},
		{
			Method:   http.MethodPut,
			Path:     "/settings",
			Handler:  okapi.H(r.h.setting.UpdateSettings),
			Group:    adminGroup,
			Summary:  "Update platform settings",
			Request:  &handlers.UpdateSettingsRequest{},
			Response: &dto.Response[[]models.Setting]{},
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
				okapi.DocErrorResponse(409, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:   http.MethodGet,
			Path:     "/plans",
			Handler:  okapi.H(r.h.plan.List),
			Group:    adminGroup,
			Summary:  "List plans",
			Request:  &handlers.ListRequest{},
			Response: &dto.PageableResponse[models.Plan]{},
		},
		{
			Method:   http.MethodGet,
			Path:     "/plans/{id:int}",
			Handler:  okapi.H(r.h.plan.Get),
			Group:    adminGroup,
			Summary:  "Get plan",
			Response: &dto.Response[models.Plan]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Plan ID"),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodPut,
			Path:        "/plans/{id:int}",
			Handler:     okapi.H(r.h.plan.Update),
			Group:       adminGroup,
			Summary:     "Update plan",
			Description: "Update a plan's configuration and limits",
			Request:     &handlers.UpdatePlanRequest{},
			Response:    &dto.Response[models.Plan]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Plan ID"),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodDelete,
			Path:        "/plans/{id:int}",
			Handler:     okapi.H(r.h.plan.Delete),
			Group:       adminGroup,
			Summary:     "Delete plan",
			Description: "Delete a plan. Use ?force=true to delete a plan assigned to workspaces.",
			Request:     &handlers.DeletePlanRequest{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Plan ID"),
				okapi.DocResponse(204, nil),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
				okapi.DocErrorResponse(409, &dto.ErrorResponseBody{}),
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
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
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
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
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
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
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
			Request:  &handlers.ListRequest{},
			Response: &dto.PageableResponse[models.Server]{},
		},
		{
			Method:   http.MethodGet,
			Path:     "/servers/{id:int}",
			Handler:  okapi.H(r.h.server.Get),
			Group:    adminGroup,
			Summary:  "Get shared SMTP server",
			Response: &dto.Response[models.Server]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Server ID"),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:   http.MethodPut,
			Path:     "/servers/{id:int}",
			Handler:  okapi.H(r.h.server.Update),
			Group:    adminGroup,
			Summary:  "Update shared SMTP server",
			Request:  &handlers.UpdateServerRequest{},
			Response: &dto.Response[models.Server]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Server ID"),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:  http.MethodDelete,
			Path:    "/servers/{id:int}",
			Handler: okapi.H(r.h.server.Delete),
			Group:   adminGroup,
			Tags:    []string{"Admin"},
			Summary: "Delete shared SMTP server",
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Server ID"),
				okapi.DocResponse(204, nil),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
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
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
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
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
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
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
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
	adminSSE := r.v1.Group("/admin", r.mw.jwtAdminQueryAuth.Middleware).WithTags([]string{"Admin"})

	return []okapi.RouteDefinition{
		{
			Method:      http.MethodGet,
			Path:        "/events/stream",
			Handler:     r.h.event.Stream,
			Group:       adminSSE,
			Summary:     "Stream events (SSE)",
			Description: "Real-time event stream via Server-Sent Events. Pass JWT token as ?token= query parameter.",
			Options:     []okapi.RouteOption{okapi.DocHide()},
		},
		{
			Method:      http.MethodGet,
			Path:        "/workers/stream",
			Handler:     r.h.admin.WorkerStream,
			Group:       adminSSE,
			Summary:     "Stream worker status (SSE)",
			Description: "Real-time worker count and details via Server-Sent Events. Pass JWT token as ?token= query parameter.",
			Options:     []okapi.RouteOption{okapi.DocHide()},
		},
	}
}
