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

	"github.com/goposta/posta/internal/dto"
	"github.com/goposta/posta/internal/handlers"
	"github.com/goposta/posta/internal/models"
	"github.com/jkaninda/okapi"
)

// userRoutes returns route definitions for all authenticated user endpoints.
func (r *Router) userRoutes() []okapi.RouteDefinition {
	userGroup := r.v1.Group("/users/me", r.mw.jwtOnly, r.mw.optionalWorkspace).WithTagInfo(okapi.GroupTag{
		Name:        tagUser,
		Description: "Manage the authenticated user: profile, password, API keys, notification preferences, and session. Requires a dashboard session — an API key cannot administer the account that issued it.",
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
				okapi.DocResponse(429, &dto.ErrorResponseBody{}),
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
				okapi.DocResponse(404, &dto.ErrorResponseBody{}),
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
			Method:      http.MethodPut,
			Path:        "/default-workspace",
			Handler:     okapi.H(r.h.user.SetDefaultWorkspace),
			Group:       userGroup,
			Summary:     "Set the default workspace",
			Description: "Chooses which workspace a request that sends no X-Posta-Workspace-Id header operates on. The caller must be a member.",
			Request:     &handlers.SetDefaultWorkspaceRequest{},
			Response:    &dto.Response[dto.MessageData]{},
			Options: []okapi.RouteOption{
				okapi.DocResponse(400, &dto.ErrorResponseBody{}),
				okapi.DocResponse(404, &dto.ErrorResponseBody{}),
			},
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
			Path:        pathSettings,
			Handler:     r.h.userSetting.GetSettings,
			Group:       userGroup,
			Summary:     "Get user settings",
			Description: "Personal notification preferences. Operational settings live at /workspaces/current/settings.",
			Response:    &dto.Response[models.UserSetting]{},
		},
		{
			Method:      http.MethodPut,
			Path:        pathSettings,
			Handler:     okapi.H(r.h.userSetting.UpdateSettings),
			Group:       userGroup,
			Summary:     "Update user settings",
			Description: "Personal notification preferences. Operational settings live at /workspaces/current/settings.",
			Request:     &handlers.UpdateUserSettingsRequest{},
			Response:    &dto.Response[models.UserSetting]{},
		},

		// In-app inbox. These sit under /users/me because a notification is
		// addressed to a person, not to a workspace: read and dismissed state
		// follow the user across every workspace they belong to.
		{
			Method:      http.MethodGet,
			Path:        pathNotifications,
			Handler:     okapi.H(r.h.notification.List),
			Group:       userGroup,
			Summary:     "List notifications",
			Description: "The authenticated user's in-app inbox, newest first. Paginate with the `before` keyset cursor.",
			Request:     &handlers.ListNotificationsRequest{},
			Response:    &dto.Response[[]models.Notification]{},
		},
		{
			Method:      http.MethodGet,
			Path:        pathNotifications + "/counts",
			Handler:     r.h.notification.Counts,
			Group:       userGroup,
			Summary:     "Get notification counts",
			Description: "Unread count for the header bell, and how many live items the active workspace has.",
			Response:    &dto.Response[handlers.NotificationCounts]{},
		},
		{
			Method:      http.MethodGet,
			Path:        pathNotifications + "/banner",
			Handler:     r.h.notification.Banner,
			Group:       userGroup,
			Summary:     "List dashboard banner notifications",
			Description: "Live, undismissed notifications for the active workspace, most severe first.",
			Response:    &dto.Response[[]models.Notification]{},
		},
		{
			Method:   http.MethodPost,
			Path:     pathNotifications + "/read",
			Handler:  okapi.H(r.h.notification.MarkRead),
			Group:    userGroup,
			Summary:  "Mark notifications read",
			Request:  &handlers.NotificationIDsRequest{},
			Response: &dto.Response[dto.MessageData]{},
		},
		{
			Method:   http.MethodPost,
			Path:     pathNotifications + "/read-all",
			Handler:  r.h.notification.MarkAllRead,
			Group:    userGroup,
			Summary:  "Mark all notifications read",
			Response: &dto.Response[dto.MessageData]{},
		},
		{
			Method:      http.MethodPost,
			Path:        pathNotifications + "/dismiss",
			Handler:     okapi.H(r.h.notification.Dismiss),
			Group:       userGroup,
			Summary:     "Dismiss notifications",
			Description: "Hides items from the dashboard banner. A dismissal is bound to the condition as it currently stands: if the condition materially changes the item resurfaces, so dismissing is not permanent silence.",
			Request:     &handlers.NotificationIDsRequest{},
			Response:    &dto.Response[dto.MessageData]{},
		},
		{
			Method:      http.MethodPost,
			Path:        pathNotifications + "/dismiss-all",
			Handler:     r.h.notification.DismissAll,
			Group:       userGroup,
			Summary:     "Dismiss all notifications",
			Description: "Clears the dashboard banner for the active workspace.",
			Response:    &dto.Response[dto.MessageData]{},
		},
	}

	return routes
}
