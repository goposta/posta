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

	"github.com/jkaninda/okapi"
	cronpkg "github.com/jkaninda/posta/internal/cron"
	"github.com/jkaninda/posta/internal/dto"
	"github.com/jkaninda/posta/internal/handlers"
	"github.com/jkaninda/posta/internal/models"
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

		// ==================== All API Keys ====================
		{
			Method:   http.MethodGet,
			Path:     "/api-keys",
			Handler:  okapi.H(r.h.admin.ListAllAPIKeys),
			Group:    adminGroup,
			Summary:  "List all API keys",
			Request:  &handlers.ListRequest{},
			Response: &dto.PageableResponse[models.APIKey]{},
		},
		{
			Method:   http.MethodDelete,
			Path:     "/api-keys/{id:int}",
			Handler:  okapi.H(r.h.admin.RevokeAPIKey),
			Group:    adminGroup,
			Summary:  "Revoke any API key",
			Response: &dto.Response[dto.MessageData]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "API key ID"),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},

		// ==================== All Emails ====================
		{
			Method:      http.MethodGet,
			Path:        "/emails",
			Handler:     okapi.H(r.h.admin.ListAllEmails),
			Group:       adminGroup,
			Summary:     "List all emails",
			Description: "List all emails across all users",
			Request:     &handlers.ListRequest{},
			Response:    &dto.PageableResponse[models.Email]{},
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
