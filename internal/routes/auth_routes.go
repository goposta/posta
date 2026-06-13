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
	"github.com/goposta/posta/internal/middlewares"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/email"
	"github.com/goposta/posta/internal/services/verifier"
	"github.com/jkaninda/okapi"
)

// healthRoutes returns route definitions for health check endpoints.
func (r *Router) healthRoutes() []okapi.RouteDefinition {
	return []okapi.RouteDefinition{
		{
			Method:   http.MethodGet,
			Path:     "/healthz",
			Handler:  r.h.health.Healthz,
			Tags:     []string{"Health"},
			Summary:  "Liveness probe",
			Response: &handlers.HealthResponse{},
		},
		{
			Method:   http.MethodGet,
			Path:     "/readyz",
			Handler:  r.h.health.Readyz,
			Tags:     []string{"Health"},
			Summary:  "Readiness probe",
			Response: &handlers.ReadyResponse{},
		},
	}
}

// infoRoute returns the route definition for the application info endpoint.
func (r *Router) infoRoute() okapi.RouteDefinition {
	return okapi.RouteDefinition{
		Method:      http.MethodGet,
		Path:        "/info",
		Handler:     handlers.Info,
		Group:       r.v1,
		Tags:        []string{"Info"},
		Summary:     "Application info",
		Description: "Returns app name, version, and commit ID",
		Response:    &dto.Response[handlers.AppInfo]{},
	}
}

// authRoutes returns route definitions for authentication endpoints.
func (r *Router) authRoutes() []okapi.RouteDefinition {
	authGroup := r.v1.Group("/auth", r.mw.loginLimiter).WithTagInfo(okapi.GroupTag{
		Name:        "Auth",
		Description: "Sign in, register, and verify email ownership. Public endpoints — protected by a per-IP login rate limiter.",
	})

	return []okapi.RouteDefinition{
		{
			Method:      http.MethodPost,
			Path:        "/login",
			Handler:     okapi.H(r.h.user.Login),
			Group:       authGroup,
			Summary:     "Login",
			Description: "Authenticate with email and password to receive a JWT token",
			Request:     &handlers.LoginRequest{},
			Response:    &dto.Response[handlers.AuthResponse]{},
			Options: []okapi.RouteOption{
				okapi.DocErrorResponse(401, &dto.ErrorResponseBody{}),
				okapi.DocErrorResponse(429, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodPost,
			Path:        "/register",
			Handler:     okapi.H(r.h.user.Register),
			Group:       authGroup,
			Summary:     "Register",
			Description: "Create a new user account (when registration is enabled)",
			Request:     &handlers.RegisterRequest{},
			Options: []okapi.RouteOption{
				okapi.DocResponse(201, &dto.Response[handlers.AuthResponse]{}),
				okapi.DocErrorResponse(403, &dto.ErrorResponseBody{}),
				okapi.DocErrorResponse(409, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodGet,
			Path:        "/registration-status",
			Handler:     r.h.user.RegistrationStatus,
			Group:       authGroup,
			Summary:     "Registration status",
			Description: "Check whether user self-registration is enabled",
			Response:    &dto.Response[any]{},
		},
		{
			Method:      http.MethodGet,
			Path:        "/verify-email",
			Handler:     okapi.H(r.h.user.VerifyEmail),
			Group:       authGroup,
			Summary:     "Verify email address",
			Description: "Redeem a verification token sent to the user's email address",
			Request:     &handlers.VerifyEmailRequest{},
			Response:    &dto.Response[any]{},
			Options: []okapi.RouteOption{
				okapi.DocErrorResponse(400, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodPost,
			Path:        "/forgot-password",
			Handler:     okapi.H(r.h.user.ForgotPassword),
			Group:       authGroup,
			Summary:     "Request password reset",
			Description: "Email a password reset link (when the feature is enabled). Always responds generically to avoid revealing whether an account exists.",
			Request:     &handlers.ForgotPasswordRequest{},
			Response:    &dto.Response[any]{},
			Options: []okapi.RouteOption{
				okapi.DocErrorResponse(400, &dto.ErrorResponseBody{}),
				okapi.DocErrorResponse(429, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodPost,
			Path:        "/reset-password",
			Handler:     okapi.H(r.h.user.ResetPassword),
			Group:       authGroup,
			Summary:     "Reset password",
			Description: "Redeem a password reset token and set a new password",
			Request:     &handlers.ResetPasswordRequest{},
			Response:    &dto.Response[any]{},
			Options: []okapi.RouteOption{
				okapi.DocErrorResponse(400, &dto.ErrorResponseBody{}),
			},
		},
	}
}

// apiAuthRoutes returns route definitions for API-key authenticated endpoints.
func (r *Router) apiAuthRoutes() []okapi.RouteDefinition {
	apiKeySec := []map[string][]string{{"ApiKeyAuth": {}}}

	// Send/subscriber routes require the `send` scope explicitly.
	apiAuth := r.v1.Group("", r.mw.apiKey, middlewares.RequireScope(models.ScopeSend)).
		WithTagInfo(okapi.GroupTag{
			Name:        "Emails",
			Description: "Send transactional and templated emails, run batch sends, and manage scheduled delivery. Authenticated with an API key.",
		}).
		WithSecurity(apiKeySec)

	// Read routes (list/get emails, bounces, webhook deliveries) require `read`.
	apiRead := r.v1.Group("", r.mw.apiKey, middlewares.RequireScope(models.ScopeRead)).
		WithTagInfo(okapi.GroupTag{
			Name:        "Read",
			Description: "Read-only access to emails, bounces, and webhook delivery logs. Authenticated with an API key holding the `read` scope.",
		}).
		WithSecurity(apiKeySec)

	// Webhook management routes require the `webhooks` scope.
	apiWebhooks := r.v1.Group("", r.mw.apiKey, middlewares.RequireScope(models.ScopeWebhooks)).
		WithTagInfo(okapi.GroupTag{
			Name:        "Webhooks",
			Description: "Create, list, and delete webhooks. Authenticated with an API key holding the `webhooks` scope.",
		}).
		WithSecurity(apiKeySec)

	return []okapi.RouteDefinition{
		{
			Method:      http.MethodPost,
			Path:        "/emails/send",
			Handler:     okapi.H(r.h.email.Send),
			Group:       apiAuth,
			Summary:     "Send an email",
			Description: "Send an email via configured SMTP server. Supports file attachments via base64-encoded content.",
			Request:     &handlers.SendEmailRequest{},
			Response:    &dto.Response[email.SendResponse]{},
			Options: []okapi.RouteOption{
				okapi.DocErrorResponse(401, &dto.ErrorResponseBody{}),
				okapi.DocErrorResponse(403, &dto.ErrorResponseBody{}),
				okapi.DocErrorResponse(429, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodPost,
			Path:        "/emails/verify",
			Handler:     okapi.H(r.h.verify.Verify),
			Group:       apiAuth,
			Summary:     "Verify an email address",
			Description: "Check whether an email address is valid/deliverable (syntax, disposable/role detection, MX records, and the caller's suppression/bounce history). Results are cached to avoid repeated lookups.",
			Request:     &handlers.VerifyAddressRequest{},
			Response:    &dto.Response[verifier.Result]{},
			Options: []okapi.RouteOption{
				okapi.DocErrorResponse(401, &dto.ErrorResponseBody{}),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
				okapi.DocErrorResponse(429, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodPost,
			Path:        "/emails/send-template",
			Handler:     okapi.H(r.h.email.SendWithTemplate),
			Group:       apiAuth,
			Summary:     "Send email using template",
			Description: "Send an email using a pre-defined template with variable substitution. Supports attachments.",
			Request:     &handlers.SendTemplateEmailRequest{},
			Response:    &dto.Response[email.SendResponse]{},
			Options: []okapi.RouteOption{
				okapi.DocErrorResponse(401, &dto.ErrorResponseBody{}),
				okapi.DocErrorResponse(403, &dto.ErrorResponseBody{}),
				okapi.DocErrorResponse(429, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodPost,
			Path:        "/emails/batch",
			Handler:     okapi.H(r.h.email.SendBatch),
			Group:       apiAuth,
			Summary:     "Send batch emails",
			Description: "Send emails to multiple recipients using a template with per-recipient variable substitution.",
			Request:     &handlers.SendBatchEmailRequest{},
			Response:    &dto.Response[email.BatchResponse]{},
			Options: []okapi.RouteOption{
				okapi.DocErrorResponse(401, &dto.ErrorResponseBody{}),
				okapi.DocErrorResponse(403, &dto.ErrorResponseBody{}),
				okapi.DocErrorResponse(429, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodPost,
			Path:        "/emails/preview",
			Handler:     okapi.H(r.h.email.Preview),
			Group:       apiAuth,
			Summary:     "Preview email from template",
			Description: "Render a template with variables and return the HTML, text, and subject without sending.",
			Request:     &handlers.PreviewEmailRequest{},
			Response:    &dto.Response[handlers.PreviewEmailResponse]{},
			Options: []okapi.RouteOption{
				okapi.DocErrorResponse(400, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodGet,
			Path:        "/emails/{id}/status",
			Handler:     okapi.H(r.h.email.GetStatus),
			Group:       apiAuth,
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
			Group:       apiAuth,
			Summary:     "Retry failed email",
			Description: "Re-enqueue a failed email for another delivery attempt. Subject to the SMTP server's max retry limit.",
			Response:    &dto.Response[email.SendResponse]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "string", "Email UUID"),
				okapi.DocErrorResponse(400, &dto.ErrorResponseBody{}),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},

		{
			Method:      http.MethodPost,
			Path:        "/subscriber-lists/{id:int}/unsubscribe",
			Handler:     okapi.H(r.h.subscriberList.UnsubscribeByEmail),
			Group:       apiAuth,
			Summary:     "Unsubscribe an email from a list",
			Description: "Opts an email out of a specific list.",
			Request:     &handlers.ListUnsubscribeByEmailRequest{},
			Response:    &dto.Response[handlers.ListSubscribeResponse]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "List ID"),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodPost,
			Path:        "/subscriber-lists/{id:int}/resubscribe",
			Handler:     okapi.H(r.h.subscriberList.ResubscribeByEmail),
			Group:       apiAuth,
			Summary:     "Re-subscribe an email to a list",
			Description: "Reverses a list-scoped opt-out and re-adds to the list (static lists only). Idempotent.",
			Request:     &handlers.ListResubscribeByEmailRequest{},
			Response:    &dto.Response[handlers.ListSubscribeResponse]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "List ID"),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodPost,
			Path:        "/subscriber-lists/subscribe",
			Handler:     okapi.H(r.h.subscriberList.Subscribe),
			Group:       apiAuth,
			Summary:     "Subscribe an email to a list",
			Description: "Adds an email to a named list, creating the list on first use. Clears any prior list-scoped opt-out for this (list, email). Idempotent.",
			Request:     &handlers.ListSubscribeRequest{},
			Response:    &dto.Response[handlers.ListSubscribeResponse]{},
			Options: []okapi.RouteOption{
				okapi.DocErrorResponse(400, &dto.ErrorResponseBody{}),
			},
		},

		//  read scope
		{
			Method:      http.MethodGet,
			Path:        "/emails",
			Handler:     okapi.H(r.h.email.List),
			Group:       apiRead,
			Summary:     "List emails",
			Description: "List sent emails with pagination. Requires the `read` scope.",
			Request:     &handlers.ListRequest{},
			Response:    &dto.PageableResponse[models.Email]{},
			Options: []okapi.RouteOption{
				okapi.DocErrorResponse(401, &dto.ErrorResponseBody{}),
				okapi.DocErrorResponse(403, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodGet,
			Path:        "/emails/{id}",
			Handler:     okapi.H(r.h.email.Get),
			Group:       apiRead,
			Summary:     "Get email details",
			Description: "Returns full details for a single email. Requires the `read` scope.",
			Response:    &dto.Response[models.Email]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "string", "Email UUID"),
				okapi.DocErrorResponse(403, &dto.ErrorResponseBody{}),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodGet,
			Path:        "/bounces",
			Handler:     okapi.H(r.h.bounce.List),
			Group:       apiRead,
			Summary:     "List bounces",
			Description: "List recorded bounces and complaints with pagination. Requires the `read` scope.",
			Request:     &handlers.ListRequest{},
			Response:    &dto.PageableResponse[models.Bounce]{},
			Options: []okapi.RouteOption{
				okapi.DocErrorResponse(401, &dto.ErrorResponseBody{}),
				okapi.DocErrorResponse(403, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodGet,
			Path:        "/webhook-deliveries",
			Handler:     okapi.H(r.h.webhookDelivery.List),
			Group:       apiRead,
			Summary:     "List webhook deliveries",
			Description: "List webhook delivery attempts with pagination. Requires the `read` scope.",
			Request:     &handlers.ListRequest{},
			Response:    &dto.PageableResponse[models.WebhookDelivery]{},
			Options: []okapi.RouteOption{
				okapi.DocErrorResponse(401, &dto.ErrorResponseBody{}),
				okapi.DocErrorResponse(403, &dto.ErrorResponseBody{}),
			},
		},

		// webhooks scope
		{
			Method:      http.MethodGet,
			Path:        "/webhooks",
			Handler:     okapi.H(r.h.webhook.List),
			Group:       apiWebhooks,
			Summary:     "List webhooks",
			Description: "List configured webhooks with pagination. Requires the `webhooks` scope.",
			Request:     &handlers.ListRequest{},
			Response:    &dto.PageableResponse[models.Webhook]{},
			Options: []okapi.RouteOption{
				okapi.DocErrorResponse(401, &dto.ErrorResponseBody{}),
				okapi.DocErrorResponse(403, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodPost,
			Path:        "/webhooks",
			Handler:     okapi.H(r.h.webhook.Create),
			Group:       apiWebhooks,
			Summary:     "Create webhook",
			Description: "Register a webhook endpoint for event delivery. Requires the `webhooks` scope.",
			Request:     &handlers.CreateWebhookRequest{},
			Options: []okapi.RouteOption{
				okapi.DocResponse(201, &dto.Response[models.Webhook]{}),
				okapi.DocErrorResponse(401, &dto.ErrorResponseBody{}),
				okapi.DocErrorResponse(403, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodDelete,
			Path:        "/webhooks/{id}",
			Handler:     okapi.H(r.h.webhook.Delete),
			Group:       apiWebhooks,
			Summary:     "Delete webhook",
			Description: "Delete a webhook by ID. Requires the `webhooks` scope.",
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Webhook ID"),
				okapi.DocResponse(204, nil),
				okapi.DocErrorResponse(403, &dto.ErrorResponseBody{}),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
	}
}
