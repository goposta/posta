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

// userRoutes returns route definitions for all authenticated user endpoints.
func (r *Router) userRoutes() []okapi.RouteDefinition {
	userGroup := r.v1.Group("/users/me", r.mw.jwtAuth.Middleware, r.mw.optionalWorkspace).WithTagInfo(okapi.GroupTag{
		Name:        "User",
		Description: "Manage the authenticated user: profile, password, API keys, notification preferences, and session.",
	})
	userGroup.WithBearerAuth()

	routes := []okapi.RouteDefinition{
		{
			Method:   http.MethodGet,
			Path:     "",
			Handler:  r.h.user.Me,
			Group:    userGroup,
			Summary:  "Get current user profile",
			Response: &dto.Response[handlers.UserProfile]{},
		},
		{
			Method:      http.MethodPut,
			Path:        "",
			Handler:     okapi.H(r.h.user.UpdateProfile),
			Group:       userGroup,
			Summary:     "Update profile",
			Description: "Update the current user's profile",
			Request:     &handlers.UpdateProfileRequest{},
			Response:    &dto.Response[handlers.UserProfile]{},
		},
		{
			Method:      http.MethodPut,
			Path:        "/password",
			Handler:     okapi.H(r.h.user.ChangePassword),
			Group:       userGroup,
			Summary:     "Change password",
			Description: "Change the current user's password",
			Request:     &handlers.ChangePasswordRequest{},
			Response:    &dto.Response[dto.MessageData]{},
		},
		{
			Method:      http.MethodPost,
			Path:        "/verify-email/resend",
			Handler:     r.h.user.ResendVerificationEmail,
			Group:       userGroup,
			Summary:     "Resend verification email",
			Description: "Issue a fresh verification email for the authenticated user",
			Response:    &dto.Response[dto.MessageData]{},
			Options: []okapi.RouteOption{
				okapi.DocErrorResponse(429, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodGet,
			Path:        "/plan",
			Handler:     r.h.plan.GetMyPlan,
			Group:       userGroup,
			Summary:     "Get my plan",
			Description: "Get the effective plan for the authenticated user",
			Response:    &dto.Response[models.Plan]{},
		},
		{
			Method:      http.MethodPost,
			Path:        "/2fa/setup",
			Handler:     r.h.user.Setup2FA,
			Group:       userGroup,
			Summary:     "Setup 2FA",
			Description: "Generate a TOTP secret for enabling 2FA. Returns secret and otpauth URL for QR code.",
			Response:    &dto.Response[handlers.Enable2FAResponse]{},
		},
		{
			Method:      http.MethodPost,
			Path:        "/2fa/verify",
			Handler:     okapi.H(r.h.user.Verify2FA),
			Group:       userGroup,
			Summary:     "Verify and enable 2FA",
			Description: "Verify a TOTP code to confirm 2FA setup",
			Request:     &handlers.Verify2FARequest{},
			Response:    &dto.Response[any]{},
		},
		{
			Method:      http.MethodPost,
			Path:        "/2fa/disable",
			Handler:     okapi.H(r.h.user.Disable2FA),
			Group:       userGroup,
			Summary:     "Disable 2FA",
			Description: "Disable 2FA after verifying a TOTP code",
			Request:     &handlers.Disable2FARequest{},
			Response:    &dto.Response[any]{},
		},
		{
			Method:      http.MethodPost,
			Path:        "/delete",
			Handler:     r.h.user.RequestAccountDeletion,
			Group:       userGroup,
			Summary:     "Request account deletion",
			Description: "Schedule account for deletion in 7 days. The account is deactivated immediately.",
			Response:    &dto.Response[any]{},
		},
		{
			Method:      http.MethodPost,
			Path:        "/cancel-deletion",
			Handler:     r.h.user.CancelAccountDeletion,
			Group:       userGroup,
			Summary:     "Cancel account deletion",
			Description: "Cancel a previously scheduled account deletion and reactivate the account.",
			Response:    &dto.Response[any]{},
		},
		{
			Method:      http.MethodGet,
			Path:        "/sessions",
			Handler:     r.h.session.List,
			Group:       userGroup,
			Summary:     "List active sessions",
			Description: "Returns all active (non-revoked, non-expired) sessions for the current user",
			Response:    &dto.Response[[]handlers.SessionResponse]{},
		},
		{
			Method:      http.MethodDelete,
			Path:        "/sessions/{id:int}",
			Handler:     okapi.H(r.h.session.Revoke),
			Group:       userGroup,
			Summary:     "Revoke session",
			Description: "Force logout a specific session by ID",
			Response:    &dto.Response[any]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Session ID"),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodPost,
			Path:        "/sessions/revoke-others",
			Handler:     r.h.session.RevokeOthers,
			Group:       userGroup,
			Summary:     "Revoke all other sessions",
			Description: "Force logout all sessions except the current one",
			Response:    &dto.Response[any]{},
		},
		{
			Method:      http.MethodPost,
			Path:        "/sessions/logout",
			Handler:     r.h.session.Logout,
			Group:       userGroup,
			Summary:     "Logout current session",
			Description: "Revoke the current session's JWT token",
			Response:    &dto.Response[any]{},
		},
		{
			Method:      http.MethodGet,
			Path:        "/audit-log",
			Handler:     okapi.H(r.h.event.UserAuditLog),
			Group:       userGroup,
			Summary:     "List user audit log",
			Description: "Returns the authenticated user's security audit trail (login, password, 2FA). Workspace-operational audit lives at /workspaces/current/audit-log.",
			Request:     &handlers.ListEventsRequest{},
			Response:    &dto.PageableResponse[models.Event]{},
		},
		{
			Method:      http.MethodGet,
			Path:        "/settings",
			Handler:     r.h.userSetting.GetSettings,
			Group:       userGroup,
			Summary:     "Get user settings",
			Description: "Personal notification preferences. Operational settings live at /workspaces/current/settings.",
			Response:    &dto.Response[models.UserSetting]{},
		},
		{
			Method:      http.MethodPut,
			Path:        "/settings",
			Handler:     okapi.H(r.h.userSetting.UpdateSettings),
			Group:       userGroup,
			Summary:     "Update user settings",
			Description: "Personal notification preferences. Operational settings live at /workspaces/current/settings.",
			Request:     &handlers.UpdateUserSettingsRequest{},
			Response:    &dto.Response[models.UserSetting]{},
		},
	}

	return routes
}
