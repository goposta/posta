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

// Authenticated by an opaque secret/token, never a JWT.
func (r *Router) inboundWebhookRoutes() []okapi.RouteDefinition {
	inboundGroup := r.app.Group("/api/v1/inbound").WithTagInfo(okapi.GroupTag{
		Name:        tagInbound,
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
			Path:    "/attachments/{uuid}/{idx}",
			Handler: okapi.H(r.h.inbound.ServeAttachment),
			Group:   inboundGroup,
			Summary: "Download an inbound email attachment (signed token)",
			Options: []okapi.RouteOption{okapi.DocHide()},
		},
	}
}

func (r *Router) inboundWorkspaceRoutes() []okapi.RouteDefinition {
	// SSE shares this group: EventSource sends the session cookie but no header; machine clients use ?token=psk_….
	userGroup := r.v1.Group("/workspaces/current", r.mw.auth, r.mw.workspaceQuery).WithTagInfo(okapi.GroupTag{
		Name:        tagInbound,
		Description: "Inbound email ingestion and retrieval: receive messages from upstream providers, browse history, and stream live events. Public ingest endpoints use opaque secrets; user endpoints use a dashboard session or an API key.",
	})
	userGroup.WithBearerAuth()

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
			Path:    "/inbound-emails/{id}/attachments/{idx}",
			Handler: okapi.H(r.h.inbound.DownloadAttachmentAuthed),
			Group:   userGroup,
			Summary: "Download an inbound email attachment (authenticated)",
			Request: &handlers.InboundAttachmentOwnedRequest{},
		},
		{
			Method:  http.MethodGet,
			Path:    "/inbound-stream",
			Handler: r.h.inbound.Stream,
			Group:   userGroup,
			Summary: "Server-sent events stream of inbound-email events",
			Options: []okapi.RouteOption{okapi.DocHide()},
		},
	}
}
