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
	"github.com/jkaninda/posta/internal/services/email"
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
	authGroup := r.v1.Group("/auth", r.mw.loginLimiter).WithTags([]string{"Auth"})

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
	}
}

// apiAuthRoutes returns route definitions for API-key authenticated endpoints.
func (r *Router) apiAuthRoutes() []okapi.RouteDefinition {
	apiAuth := r.v1.Group("", r.mw.apiKey).WithTags([]string{"Emails"})
	apiAuth.WithBearerAuth()

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
	}
}
