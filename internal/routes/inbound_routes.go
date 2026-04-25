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

// inboundWebhookRoutes returns public routes for the generic inbound-email webhook
// and signed attachment downloads. Both are auth'd by opaque secret/token — no JWT.
func (r *Router) inboundWebhookRoutes() []okapi.RouteDefinition {
	inboundGroup := r.app.Group("/api/v1/inbound").WithTagInfo(okapi.GroupTag{
		Name:        "Inbound",
		Description: "Inbound email ingestion and retrieval: receive messages from upstream providers, browse history, and stream live events.",
	})
	return []okapi.RouteDefinition{
		{
			Method:   http.MethodPost,
			Path:     "/webhook",
			Handler:  okapi.H(r.h.inbound.Receive),
			Group:    inboundGroup,
			Summary:  "Receive inbound email via webhook",
			Request:  &handlers.InboundWebhookRequest{},
			Response: &handlers.InboundWebhookResponse{},
		},
		{
			Method:  http.MethodGet,
			Path:    "/attachments/{uuid}/{idx:int}",
			Handler: okapi.H(r.h.inbound.ServeAttachment),
			Group:   inboundGroup,
			Summary: "Download an inbound email attachment (signed token)",
			Options: []okapi.RouteOption{okapi.DocHide()},
		},
	}
}

// inboundUserRoutes returns authenticated routes for browsing inbound email history.
func (r *Router) inboundUserRoutes() []okapi.RouteDefinition {
	userGroup := r.v1.Group("/users/me", r.mw.jwtAuth.Middleware, r.mw.optionalWorkspace).WithTagInfo(okapi.GroupTag{
		Name:        "Inbound",
		Description: "Inbound email ingestion and retrieval: receive messages from upstream providers, browse history, and stream live events. Public ingest endpoints use opaque secrets; user endpoints use JWT.",
	})
	userGroup.WithBearerAuth()

	// SSE endpoints use query-param auth because browsers cannot set custom
	// Authorization headers on EventSource connections.
	streamGroup := r.v1.Group("/users/me", r.mw.jwtQueryAuth.Middleware, r.mw.optionalWorkspace).WithTagInfo(okapi.GroupTag{
		Name:        "Inbound",
		Description: "Inbound email ingestion and retrieval: receive messages from upstream providers, browse history, and stream live events. Public ingest endpoints use opaque secrets; user endpoints use JWT.",
	})

	return []okapi.RouteDefinition{
		{
			Method:   http.MethodGet,
			Path:     "/inbound-emails",
			Handler:  okapi.H(r.h.inbound.List),
			Group:    userGroup,
			Summary:  "List inbound emails",
			Request:  &handlers.InboundListRequest{},
			Response: &dto.PageableResponse[models.InboundEmail]{},
		},
		{
			Method:   http.MethodGet,
			Path:     "/inbound-emails/{id}",
			Handler:  okapi.H(r.h.inbound.Get),
			Group:    userGroup,
			Summary:  "Get an inbound email by UUID",
			Request:  &handlers.GetEmailRequest{},
			Response: &dto.Response[models.InboundEmail]{},
			Options:  []okapi.RouteOption{okapi.DocPathParam("id", "string", "Inbound email UUID")},
		},
		{
			Method:  http.MethodDelete,
			Path:    "/inbound-emails/{id}",
			Handler: okapi.H(r.h.inbound.Delete),
			Group:   userGroup,
			Summary: "Delete an inbound email",
			Request: &handlers.GetEmailRequest{},
			Options: []okapi.RouteOption{okapi.DocPathParam("id", "string", "Inbound email UUID")},
		},
		{
			Method:  http.MethodPost,
			Path:    "/inbound-emails/{id}/retry",
			Handler: okapi.H(r.h.inbound.Retry),
			Group:   userGroup,
			Summary: "Retry webhook dispatch for a failed inbound email",
			Request: &handlers.GetEmailRequest{},
			Options: []okapi.RouteOption{okapi.DocPathParam("id", "string", "Inbound email UUID")},
		},
		{
			Method:  http.MethodGet,
			Path:    "/inbound-emails/{id}/raw",
			Handler: okapi.H(r.h.inbound.GetRaw),
			Group:   userGroup,
			Summary: "Download the raw RFC 5322 message (.eml)",
			Request: &handlers.GetEmailRequest{},
			Options: []okapi.RouteOption{okapi.DocPathParam("id", "string", "Inbound email UUID")},
		},
		{
			Method:  http.MethodGet,
			Path:    "/inbound-emails/{uuid}/attachments/{idx:int}",
			Handler: okapi.H(r.h.inbound.DownloadAttachmentAuthed),
			Group:   userGroup,
			Summary: "Download an inbound email attachment (authenticated)",
			Request: &handlers.InboundAttachmentOwnedRequest{},
		},
		{
			Method:  http.MethodGet,
			Path:    "/inbound-stream",
			Handler: r.h.inbound.Stream,
			Group:   streamGroup,
			Summary: "Server-sent events stream of inbound-email events",
			Options: []okapi.RouteOption{okapi.DocHide()},
		},
	}
}
