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
	"github.com/jkaninda/posta/internal/dto"
	"github.com/jkaninda/posta/internal/handlers"
	"github.com/jkaninda/posta/internal/models"
	"github.com/jkaninda/posta/internal/services/email"
)

// userRoutes returns route definitions for all authenticated user endpoints.
func (r *Router) userRoutes() []okapi.RouteDefinition {
	userGroup := r.v1.Group("/users/me", r.mw.jwtAuth.Middleware).WithTags([]string{"User"})
	userGroup.WithBearerAuth()

	return []okapi.RouteDefinition{
		// ==================== Profile ====================
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

		// ==================== 2FA ====================
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

		// ==================== Sessions ====================
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

		// ==================== Dashboard & Analytics ====================
		{
			Method:      http.MethodGet,
			Path:        "/dashboard/stats",
			Handler:     r.h.dashboard.Stats,
			Group:       userGroup,
			Summary:     "Get dashboard statistics",
			Description: "Returns email sending statistics for the authenticated user",
			Response:    &dto.Response[handlers.DashboardStats]{},
		},
		{
			Method:      http.MethodGet,
			Path:        "/analytics",
			Handler:     okapi.H(r.h.analytics.UserAnalytics),
			Group:       userGroup,
			Summary:     "Email analytics",
			Description: "Returns daily email counts and status breakdown for the authenticated user",
			Request:     &handlers.AnalyticsRequest{},
			Response:    &dto.Response[handlers.AnalyticsResponse]{},
		},
		{
			Method:      http.MethodGet,
			Path:        "/analytics/dashboard",
			Handler:     okapi.H(r.h.analytics.UserDashboardAnalytics),
			Group:       userGroup,
			Summary:     "Dashboard analytics",
			Description: "Returns delivery rate trends, bounce rate graphs, and latency percentiles",
			Request:     &handlers.DashboardAnalyticsRequest{},
			Response:    &dto.Response[handlers.DashboardAnalyticsResponse]{},
		},

		// ==================== Email Logs ====================
		{
			Method:      http.MethodGet,
			Path:        "/emails",
			Handler:     okapi.H(r.h.email.List),
			Group:       userGroup,
			Summary:     "List emails",
			Description: "List sent emails with pagination",
			Request:     &handlers.ListRequest{},
			Response:    &dto.PageableResponse[models.Email]{},
		},
		{
			Method:   http.MethodGet,
			Path:     "/emails/{id}",
			Handler:  okapi.H(r.h.email.Get),
			Group:    userGroup,
			Summary:  "Get email details",
			Response: &dto.Response[models.Email]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "string", "Email UUID"),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodGet,
			Path:        "/emails/{id}/status",
			Handler:     okapi.H(r.h.email.GetStatus),
			Group:       userGroup,
			Summary:     "Get email delivery status",
			Description: "Returns a lightweight status view for polling email delivery progress. Only the email owner can access this.",
			Response:    &dto.Response[handlers.EmailStatusResponse]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "string", "Email UUID"),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodPost,
			Path:        "/emails/{id}/retry",
			Handler:     okapi.H(r.h.email.Retry),
			Group:       userGroup,
			Summary:     "Retry failed email",
			Description: "Re-enqueue a failed email for another delivery attempt",
			Response:    &dto.Response[email.SendResponse]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "string", "Email UUID"),
				okapi.DocErrorResponse(400, &dto.ErrorResponseBody{}),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},

		// ==================== Email Preview ====================
		{
			Method:      http.MethodPost,
			Path:        "/emails/preview",
			Handler:     okapi.H(r.h.email.Preview),
			Group:       userGroup,
			Summary:     "Preview email from template",
			Description: "Render a template with variables and return the HTML, text, and subject without sending.",
			Request:     &handlers.PreviewEmailRequest{},
			Response:    &dto.Response[handlers.PreviewEmailResponse]{},
			Options: []okapi.RouteOption{
				okapi.DocErrorResponse(400, &dto.ErrorResponseBody{}),
			},
		},

		// ==================== API Keys ====================
		{
			Method:      http.MethodPost,
			Path:        "/api-keys",
			Handler:     okapi.H(r.h.apiKey.Create),
			Group:       userGroup,
			Summary:     "Create API key",
			Description: "Generate a new API key. The raw key is only shown once in the response.",
			Request:     &handlers.CreateAPIKeyRequest{},
			Options: []okapi.RouteOption{
				okapi.DocResponse(201, &dto.Response[dto.APIKeyCreatedData]{}),
			},
		},
		{
			Method:   http.MethodGet,
			Path:     "/api-keys",
			Handler:  okapi.H(r.h.apiKey.List),
			Group:    userGroup,
			Summary:  "List API keys",
			Request:  &handlers.ListRequest{},
			Response: &dto.PageableResponse[models.APIKey]{},
		},
		{
			Method:   http.MethodPut,
			Path:     "/api-keys/{id:int}/revoke",
			Handler:  okapi.H(r.h.apiKey.Revoke),
			Group:    userGroup,
			Summary:  "Revoke API key",
			Response: &dto.Response[dto.MessageData]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "API key ID"),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:   http.MethodDelete,
			Path:     "/api-keys/{id:int}",
			Handler:  okapi.H(r.h.apiKey.Delete),
			Group:    userGroup,
			Summary:  "Delete API key",
			Response: &dto.Response[dto.MessageData]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "API key ID"),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},

		// ==================== Templates ====================
		{
			Method:  http.MethodPost,
			Path:    "/templates",
			Handler: okapi.H(r.h.template.Create),
			Group:   userGroup,
			Tags:    []string{"User"},
			Summary: "Create template",
			Request: &handlers.CreateTemplateRequest{},
			Options: []okapi.RouteOption{
				okapi.DocResponse(201, &dto.Response[models.Template]{}),
				okapi.DocErrorResponse(409, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:   http.MethodGet,
			Path:     "/templates",
			Handler:  okapi.H(r.h.template.List),
			Group:    userGroup,
			Summary:  "List templates",
			Request:  &handlers.ListRequest{},
			Response: &dto.PageableResponse[models.Template]{},
		},
		{
			Method:   http.MethodPut,
			Path:     "/templates/{id:int}",
			Handler:  okapi.H(r.h.template.Update),
			Group:    userGroup,
			Summary:  "Update template",
			Request:  &handlers.UpdateTemplateRequest{},
			Response: &dto.Response[models.Template]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Template ID"),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:  http.MethodDelete,
			Path:    "/templates/{id:int}",
			Handler: okapi.H(r.h.template.Delete),
			Group:   userGroup,
			Tags:    []string{"User"},
			Summary: "Delete template",
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Template ID"),
				okapi.DocResponse(204, nil),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodPost,
			Path:        "/templates/preview",
			Handler:     okapi.H(r.h.template.Preview),
			Group:       userGroup,
			Summary:     "Preview template",
			Description: "Render a template with sample data and return the result",
			Request:     &handlers.PreviewTemplateRequest{},
			Response:    &dto.Response[handlers.PreviewResult]{},
		},
		{
			Method:      http.MethodPost,
			Path:        "/templates/{id:int}/send-test",
			Handler:     okapi.H(r.h.template.SendTest),
			Group:       userGroup,
			Summary:     "Send test email",
			Description: "Send a test email using a template with sample data",
			Request:     &handlers.SendTestRequest{},
			Response:    &dto.Response[email.SendResponse]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Template ID"),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},

		// ==================== Template Import/Export ====================
		{
			Method:      http.MethodGet,
			Path:        "/templates/{id:int}/export",
			Handler:     okapi.H(r.h.template.Export),
			Group:       userGroup,
			Summary:     "Export template",
			Description: "Export a template with all versions and localizations as JSON",
			Response:    &dto.Response[handlers.TemplateExport]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Template ID"),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodPost,
			Path:        "/templates/import",
			Handler:     okapi.H(r.h.template.Import),
			Group:       userGroup,
			Summary:     "Import template",
			Description: "Import a template from a previously exported JSON payload",
			Request:     &handlers.ImportTemplateRequest{},
			Options: []okapi.RouteOption{
				okapi.DocResponse(201, &dto.Response[models.Template]{}),
				okapi.DocErrorResponse(409, &dto.ErrorResponseBody{}),
			},
		},

		// ==================== Template Versions ====================
		{
			Method:   http.MethodGet,
			Path:     "/templates/{id:int}/versions",
			Handler:  okapi.H(r.h.version.List),
			Group:    userGroup,
			Tags:     []string{"Template Versions"},
			Summary:  "List template versions",
			Response: &dto.Response[[]models.TemplateVersion]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Template ID"),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:  http.MethodPost,
			Path:    "/templates/{id:int}/versions",
			Handler: okapi.H(r.h.version.Create),
			Group:   userGroup,
			Tags:    []string{"Template Versions"},
			Summary: "Create template version",
			Request: &handlers.CreateVersionRequest{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Template ID"),
				okapi.DocResponse(201, &dto.Response[models.TemplateVersion]{}),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:   http.MethodPut,
			Path:     "/templates/{id:int}/versions/{versionId:int}",
			Handler:  okapi.H(r.h.version.Update),
			Group:    userGroup,
			Tags:     []string{"Template Versions"},
			Summary:  "Update template version",
			Request:  &handlers.UpdateVersionRequest{},
			Response: &dto.Response[models.TemplateVersion]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Template ID"),
				okapi.DocPathParam("versionId", "integer", "Version ID"),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:   http.MethodPost,
			Path:     "/templates/{id:int}/activate/{versionId:int}",
			Handler:  okapi.H(r.h.version.Activate),
			Group:    userGroup,
			Tags:     []string{"Template Versions"},
			Summary:  "Activate template version",
			Response: &dto.Response[models.Template]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Template ID"),
				okapi.DocPathParam("versionId", "integer", "Version ID"),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:  http.MethodDelete,
			Path:    "/templates/{id:int}/versions/{versionId:int}",
			Handler: okapi.H(r.h.version.Delete),
			Group:   userGroup,
			Tags:    []string{"Template Versions"},
			Summary: "Delete template version",
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Template ID"),
				okapi.DocPathParam("versionId", "integer", "Version ID"),
				okapi.DocResponse(204, nil),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},

		// ==================== Template Localizations ====================
		{
			Method:   http.MethodGet,
			Path:     "/templates/{id:int}/versions/{versionId:int}/localizations",
			Handler:  okapi.H(r.h.localization.List),
			Group:    userGroup,
			Tags:     []string{"Template Localizations"},
			Summary:  "List localizations for a version",
			Response: &dto.Response[[]models.TemplateLocalization]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Template ID"),
				okapi.DocPathParam("versionId", "integer", "Version ID"),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:  http.MethodPost,
			Path:    "/templates/{id:int}/versions/{versionId:int}/localizations",
			Handler: okapi.H(r.h.localization.Create),
			Group:   userGroup,
			Tags:    []string{"Template Localizations"},
			Summary: "Create localization",
			Request: &handlers.CreateLocalizationRequest{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Template ID"),
				okapi.DocPathParam("versionId", "integer", "Version ID"),
				okapi.DocResponse(201, &dto.Response[models.TemplateLocalization]{}),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
				okapi.DocErrorResponse(409, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:   http.MethodPut,
			Path:     "/localizations/{localizationId:int}",
			Handler:  okapi.H(r.h.localization.Update),
			Group:    userGroup,
			Tags:     []string{"Template Localizations"},
			Summary:  "Update localization",
			Request:  &handlers.UpdateLocalizationRequest{},
			Response: &dto.Response[models.TemplateLocalization]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("localizationId", "integer", "Localization ID"),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:  http.MethodDelete,
			Path:    "/localizations/{localizationId:int}",
			Handler: okapi.H(r.h.localization.Delete),
			Group:   userGroup,
			Tags:    []string{"Template Localizations"},
			Summary: "Delete localization",
			Options: []okapi.RouteOption{
				okapi.DocPathParam("localizationId", "integer", "Localization ID"),
				okapi.DocResponse(204, nil),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodPost,
			Path:        "/templates/{id:int}/versions/{versionId:int}/preview",
			Handler:     okapi.H(r.h.localization.Preview),
			Group:       userGroup,
			Tags:        []string{"Template Localizations"},
			Summary:     "Preview localized template",
			Description: "Render a specific language version of a template with sample data",
			Request:     &handlers.PreviewLocalizationRequest{},
			Response:    &dto.Response[handlers.PreviewResult]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Template ID"),
				okapi.DocPathParam("versionId", "integer", "Version ID"),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},

		// ==================== Languages ====================
		{
			Method:  http.MethodPost,
			Path:    "/languages",
			Handler: okapi.H(r.h.language.Create),
			Group:   userGroup,
			Tags:    []string{"Languages"},
			Summary: "Create language",
			Request: &handlers.CreateLanguageRequest{},
			Options: []okapi.RouteOption{
				okapi.DocResponse(201, &dto.Response[models.Language]{}),
				okapi.DocErrorResponse(409, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:   http.MethodGet,
			Path:     "/languages",
			Handler:  okapi.H(r.h.language.List),
			Group:    userGroup,
			Tags:     []string{"Languages"},
			Summary:  "List languages",
			Request:  &handlers.ListRequest{},
			Response: &dto.PageableResponse[models.Language]{},
		},
		{
			Method:   http.MethodPut,
			Path:     "/languages/{id:int}",
			Handler:  okapi.H(r.h.language.Update),
			Group:    userGroup,
			Tags:     []string{"Languages"},
			Summary:  "Update language",
			Request:  &handlers.UpdateLanguageRequest{},
			Response: &dto.Response[models.Language]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Language ID"),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:  http.MethodDelete,
			Path:    "/languages/{id:int}",
			Handler: okapi.H(r.h.language.Delete),
			Group:   userGroup,
			Tags:    []string{"Languages"},
			Summary: "Delete language",
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Language ID"),
				okapi.DocResponse(204, nil),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},

		// ==================== Stylesheets ====================
		{
			Method:  http.MethodPost,
			Path:    "/stylesheets",
			Handler: okapi.H(r.h.stylesheet.Create),
			Group:   userGroup,
			Tags:    []string{"User"},
			Summary: "Create stylesheet",
			Request: &handlers.CreateStyleSheetRequest{},
			Options: []okapi.RouteOption{
				okapi.DocResponse(201, &dto.Response[models.StyleSheet]{}),
				okapi.DocErrorResponse(409, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:   http.MethodGet,
			Path:     "/stylesheets",
			Handler:  okapi.H(r.h.stylesheet.List),
			Group:    userGroup,
			Summary:  "List stylesheets",
			Request:  &handlers.ListRequest{},
			Response: &dto.PageableResponse[models.StyleSheet]{},
		},
		{
			Method:   http.MethodPut,
			Path:     "/stylesheets/{id:int}",
			Handler:  okapi.H(r.h.stylesheet.Update),
			Group:    userGroup,
			Summary:  "Update stylesheet",
			Request:  &handlers.UpdateStyleSheetRequest{},
			Response: &dto.Response[models.StyleSheet]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "StyleSheet ID"),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:  http.MethodDelete,
			Path:    "/stylesheets/{id:int}",
			Handler: okapi.H(r.h.stylesheet.Delete),
			Group:   userGroup,
			Tags:    []string{"User"},
			Summary: "Delete stylesheet",
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "StyleSheet ID"),
				okapi.DocResponse(204, nil),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},

		// ==================== SMTP Servers ====================
		{
			Method:  http.MethodPost,
			Path:    "/smtp-servers",
			Handler: okapi.H(r.h.smtp.Create),
			Group:   userGroup,
			Tags:    []string{"User"},
			Summary: "Add SMTP server",
			Request: &handlers.CreateSMTPRequest{},
			Options: []okapi.RouteOption{
				okapi.DocResponse(201, &dto.Response[models.SMTPServer]{}),
			},
		},
		{
			Method:   http.MethodGet,
			Path:     "/smtp-servers",
			Handler:  okapi.H(r.h.smtp.List),
			Group:    userGroup,
			Summary:  "List SMTP servers",
			Request:  &handlers.ListRequest{},
			Response: &dto.PageableResponse[models.SMTPServer]{},
		},
		{
			Method:   http.MethodGet,
			Path:     "/smtp-servers/{id:int}",
			Handler:  okapi.H(r.h.smtp.Get),
			Group:    userGroup,
			Summary:  "Get SMTP server",
			Request:  &handlers.GetSMTPRequest{},
			Response: &dto.Response[models.SMTPServer]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "SMTP server ID"),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:   http.MethodPut,
			Path:     "/smtp-servers/{id:int}",
			Handler:  okapi.H(r.h.smtp.Update),
			Group:    userGroup,
			Summary:  "Update SMTP server",
			Request:  &handlers.UpdateSMTPRequest{},
			Response: &dto.Response[models.SMTPServer]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "SMTP server ID"),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:  http.MethodDelete,
			Path:    "/smtp-servers/{id:int}",
			Handler: okapi.H(r.h.smtp.Delete),
			Group:   userGroup,
			Tags:    []string{"User"},
			Summary: "Delete SMTP server",
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "SMTP server ID"),
				okapi.DocResponse(204, nil),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:   http.MethodPost,
			Path:     "/smtp-servers/{id:int}/test",
			Handler:  okapi.H(r.h.smtp.Test),
			Group:    userGroup,
			Summary:  "Test SMTP server connection",
			Response: &dto.Response[dto.MessageData]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "SMTP server ID"),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},

		// ==================== Webhooks ====================
		{
			Method:  http.MethodPost,
			Path:    "/webhooks",
			Handler: okapi.H(r.h.webhook.Create),
			Group:   userGroup,
			Tags:    []string{"User"},
			Summary: "Create webhook",
			Request: &handlers.CreateWebhookRequest{},
			Options: []okapi.RouteOption{
				okapi.DocResponse(201, &dto.Response[models.Webhook]{}),
			},
		},
		{
			Method:   http.MethodGet,
			Path:     "/webhooks",
			Handler:  okapi.H(r.h.webhook.List),
			Group:    userGroup,
			Summary:  "List webhooks",
			Request:  &handlers.ListRequest{},
			Response: &dto.PageableResponse[models.Webhook]{},
		},
		{
			Method:  http.MethodDelete,
			Path:    "/webhooks/{id:int}",
			Handler: okapi.H(r.h.webhook.Delete),
			Group:   userGroup,
			Tags:    []string{"User"},
			Summary: "Delete webhook",
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Webhook ID"),
				okapi.DocResponse(204, nil),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},

		// ==================== Webhook Deliveries ====================
		{
			Method:   http.MethodGet,
			Path:     "/webhook-deliveries",
			Handler:  okapi.H(r.h.webhookDelivery.List),
			Group:    userGroup,
			Summary:  "List webhook deliveries",
			Request:  &handlers.ListRequest{},
			Response: &dto.PageableResponse[models.WebhookDelivery]{},
		},

		// ==================== Domains ====================
		{
			Method:      http.MethodPost,
			Path:        "/domains",
			Handler:     okapi.H(r.h.domain.Create),
			Group:       userGroup,
			Summary:     "Add domain",
			Description: "Register a domain for verification. Returns the DNS records that must be configured.",
			Request:     &handlers.CreateDomainRequest{},
			Options: []okapi.RouteOption{
				okapi.DocResponse(201, &dto.Response[handlers.DomainWithRecords]{}),
				okapi.DocErrorResponse(409, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:   http.MethodGet,
			Path:     "/domains",
			Handler:  okapi.H(r.h.domain.List),
			Group:    userGroup,
			Summary:  "List domains",
			Request:  &handlers.ListRequest{},
			Response: &dto.PageableResponse[models.Domain]{},
		},
		{
			Method:   http.MethodGet,
			Path:     "/domains/{id:int}",
			Handler:  okapi.H(r.h.domain.Get),
			Group:    userGroup,
			Summary:  "Get domain details",
			Response: &dto.Response[handlers.DomainWithRecords]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Domain ID"),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:  http.MethodDelete,
			Path:    "/domains/{id:int}",
			Handler: okapi.H(r.h.domain.Delete),
			Group:   userGroup,
			Tags:    []string{"User"},
			Summary: "Delete domain",
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Domain ID"),
				okapi.DocResponse(204, nil),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodPost,
			Path:        "/domains/{id:int}/verify",
			Handler:     okapi.H(r.h.domain.Verify),
			Group:       userGroup,
			Summary:     "Verify domain DNS records",
			Description: "Check DNS records (SPF, DKIM, DMARC) for the domain",
			Response:    &dto.Response[any]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Domain ID"),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},

		// ==================== Bounces ====================
		{
			Method:      http.MethodPost,
			Path:        "/bounces",
			Handler:     okapi.H(r.h.bounce.Record),
			Group:       userGroup,
			Summary:     "Record a bounce",
			Description: "Record a bounce or complaint. Hard bounces and complaints auto-suppress the recipient.",
			Request:     &handlers.RecordBounceRequest{},
			Options: []okapi.RouteOption{
				okapi.DocResponse(201, &dto.Response[models.Bounce]{}),
			},
		},
		{
			Method:   http.MethodGet,
			Path:     "/bounces",
			Handler:  okapi.H(r.h.bounce.List),
			Group:    userGroup,
			Summary:  "List bounces",
			Request:  &handlers.ListRequest{},
			Response: &dto.PageableResponse[models.Bounce]{},
		},

		// ==================== Suppressions ====================
		{
			Method:  http.MethodPost,
			Path:    "/suppressions",
			Handler: okapi.H(r.h.suppression.Create),
			Group:   userGroup,
			Tags:    []string{"User"},
			Summary: "Add to suppression list",
			Request: &handlers.CreateSuppressionRequest{},
			Options: []okapi.RouteOption{
				okapi.DocResponse(201, &dto.Response[models.Suppression]{}),
				okapi.DocErrorResponse(409, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:   http.MethodGet,
			Path:     "/suppressions",
			Handler:  okapi.H(r.h.suppression.List),
			Group:    userGroup,
			Summary:  "List suppressed emails",
			Request:  &handlers.ListRequest{},
			Response: &dto.PageableResponse[models.Suppression]{},
		},
		{
			Method:  http.MethodDelete,
			Path:    "/suppressions",
			Handler: okapi.H(r.h.suppression.Delete),
			Group:   userGroup,
			Tags:    []string{"User"},
			Summary: "Remove from suppression list",
			Request: &handlers.DeleteSuppressionRequest{},
			Options: []okapi.RouteOption{
				okapi.DocResponse(204, nil),
			},
		},

		// ==================== Audit Log ====================
		{
			Method:      http.MethodGet,
			Path:        "/audit-log",
			Handler:     okapi.H(r.h.event.UserAuditLog),
			Group:       userGroup,
			Summary:     "List audit log",
			Description: "Returns the authenticated user's audit trail",
			Request:     &handlers.ListEventsRequest{},
			Response:    &dto.PageableResponse[models.Event]{},
		},

		// ==================== Contacts ====================
		{
			Method:      http.MethodGet,
			Path:        "/contacts",
			Handler:     okapi.H(r.h.contact.List),
			Group:       userGroup,
			Summary:     "List contacts",
			Description: "List all recipient email addresses with sent/failed counts. Use the search query parameter to filter by email or name.",
			Request:     &handlers.ListContactsRequest{},
			Response:    &dto.PageableResponse[models.Contact]{},
		},
		{
			Method:      http.MethodGet,
			Path:        "/contacts/{id:int}",
			Handler:     okapi.H(r.h.contact.Get),
			Group:       userGroup,
			Summary:     "Get contact details",
			Description: "Get a single contact by ID with suppression status",
			Request:     &handlers.GetByIDRequest{},
			Response:    &dto.Response[models.Contact]{},
		},

		// ==================== Contact Lists ====================
		{
			Method:  http.MethodPost,
			Path:    "/contact-lists",
			Handler: okapi.H(r.h.contactList.Create),
			Group:   userGroup,
			Tags:    []string{"Contact Lists"},
			Summary: "Create contact list",
			Request: &handlers.CreateContactListRequest{},
			Options: []okapi.RouteOption{
				okapi.DocResponse(201, &dto.Response[models.ContactList]{}),
			},
		},
		{
			Method:   http.MethodGet,
			Path:     "/contact-lists",
			Handler:  okapi.H(r.h.contactList.List),
			Group:    userGroup,
			Tags:     []string{"Contact Lists"},
			Summary:  "List contact lists",
			Request:  &handlers.ListRequest{},
			Response: &dto.PageableResponse[handlers.ContactListWithCount]{},
		},
		{
			Method:   http.MethodPut,
			Path:     "/contact-lists/{id:int}",
			Handler:  okapi.H(r.h.contactList.Update),
			Group:    userGroup,
			Tags:     []string{"Contact Lists"},
			Summary:  "Update contact list",
			Request:  &handlers.UpdateContactListRequest{},
			Response: &dto.Response[models.ContactList]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Contact list ID"),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:  http.MethodDelete,
			Path:    "/contact-lists/{id:int}",
			Handler: okapi.H(r.h.contactList.Delete),
			Group:   userGroup,
			Tags:    []string{"Contact Lists"},
			Summary: "Delete contact list",
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Contact list ID"),
				okapi.DocResponse(204, nil),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:  http.MethodPost,
			Path:    "/contact-lists/{id:int}/members",
			Handler: okapi.H(r.h.contactList.AddMember),
			Group:   userGroup,
			Tags:    []string{"Contact Lists"},
			Summary: "Add member to list",
			Request: &handlers.AddMemberRequest{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Contact list ID"),
				okapi.DocResponse(201, &dto.Response[models.ContactListMember]{}),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
				okapi.DocErrorResponse(409, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:  http.MethodDelete,
			Path:    "/contact-lists/{id:int}/members",
			Handler: okapi.H(r.h.contactList.RemoveMember),
			Group:   userGroup,
			Tags:    []string{"Contact Lists"},
			Summary: "Remove member from list",
			Request: &handlers.RemoveMemberRequest{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Contact list ID"),
				okapi.DocResponse(204, nil),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:   http.MethodGet,
			Path:     "/contact-lists/{id:int}/members",
			Handler:  okapi.H(r.h.contactList.ListMembers),
			Group:    userGroup,
			Tags:     []string{"Contact Lists"},
			Summary:  "List members in contact list",
			Request:  &handlers.ListMembersRequest{},
			Response: &dto.PageableResponse[models.ContactListMember]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Contact list ID"),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},

		// ==================== User Data Export/Import ====================
		{
			Method:      http.MethodGet,
			Path:        "/data/export",
			Handler:     r.h.userData.Export,
			Group:       userGroup,
			Summary:     "Export all user data",
			Description: "Export all user data including templates, stylesheets, languages, contacts, contact lists, webhooks, suppressions, and settings as JSON",
			Response:    &dto.Response[handlers.UserDataExport]{},
		},
		{
			Method:      http.MethodPost,
			Path:        "/data/import",
			Handler:     okapi.H(r.h.userData.Import),
			Group:       userGroup,
			Summary:     "Import user data",
			Description: "Import user data from a previously exported JSON payload. Duplicates are skipped.",
			Request:     &handlers.ImportUserDataRequest{},
			Response:    &dto.Response[any]{},
		},

		// ==================== GDPR Data Management ====================
		{
			Method:      http.MethodPost,
			Path:        "/gdpr/delete-contacts",
			Handler:     okapi.H(r.h.userData.DeleteContacts),
			Group:       userGroup,
			Summary:     "Delete contact data (GDPR)",
			Description: "Delete a specific contact by email or all contacts. Also removes the contact from suppression lists and contact list memberships.",
			Request:     &handlers.GDPRDeleteContactsRequest{},
			Response:    &dto.Response[handlers.GDPRDeleteResult]{},
		},
		{
			Method:      http.MethodPost,
			Path:        "/gdpr/delete-email-logs",
			Handler:     okapi.H(r.h.userData.DeleteEmailLogs),
			Group:       userGroup,
			Summary:     "Delete email logs (GDPR)",
			Description: "Delete email logs and associated bounces. Specify older_than_days to only delete old logs, or 0 to delete all.",
			Request:     &handlers.GDPRDeleteEmailLogsRequest{},
			Response:    &dto.Response[handlers.GDPRDeleteResult]{},
		},

		// ==================== User Settings ====================
		{
			Method:   http.MethodGet,
			Path:     "/settings",
			Handler:  r.h.userSetting.GetSettings,
			Group:    userGroup,
			Summary:  "Get user settings",
			Response: &dto.Response[models.UserSetting]{},
		},
		{
			Method:   http.MethodPut,
			Path:     "/settings",
			Handler:  okapi.H(r.h.userSetting.UpdateSettings),
			Group:    userGroup,
			Summary:  "Update user settings",
			Request:  &handlers.UpdateUserSettingsRequest{},
			Response: &dto.Response[models.UserSetting]{},
		},
	}
}
