// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package routes

import (
	"net/http"

	"github.com/goposta/posta/internal/dto"
	"github.com/goposta/posta/internal/handlers"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/messages"
	"github.com/jkaninda/okapi"
)

func (r *Router) formIngestRoutes() []okapi.RouteDefinition {
	ingestGroup := r.app.Group("/api/v1/f").WithTagInfo(okapi.GroupTag{
		Name:        "Forms",
		Description: "Public form ingest. A website form posts here with the form's public key; no authentication is used, and access is governed by the form's origin allowlist, honeypot, nonce, and rate limits.",
	})

	return []okapi.RouteDefinition{
		{
			Method:      http.MethodPost,
			Path:        "/{key}",
			Handler:     r.h.formIngest.Submit,
			Group:       ingestGroup,
			Summary:     "Submit a web form",
			Description: "Accepts application/json, text/plain holding JSON, application/x-www-form-urlencoded, and multipart/form-data. Always answers 202 for a stored or silently rejected submission so spam clients learn nothing.",
			Options: []okapi.RouteOption{
				okapi.DocPathParam("key", "string", "Form public key"),
				okapi.DocErrorResponse(403, &dto.ErrorResponseBody{}),
				okapi.DocErrorResponse(429, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodGet,
			Path:        "/{key}/nonce",
			Handler:     r.h.formIngest.Nonce,
			Group:       ingestGroup,
			Summary:     "Issue a submission nonce",
			Description: "Returns a short-lived signed nonce for forms that require one. Single-use and bound to a minimum fill time.",
			Response:    &dto.Response[messages.Nonce]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("key", "string", "Form public key"),
			},
		},
	}
}

func (r *Router) messageWorkspaceRoutes() []okapi.RouteDefinition {
	opsGroup := r.v1.Group("/workspaces/current", r.mw.auth, r.mw.workspace).WithTagInfo(okapi.GroupTag{
		Name:        "Messages",
		Description: "Web form definitions, received messages, operator replies, and spam filters for the active workspace. Authenticated with a dashboard session or an API key bound to the workspace.",
	})
	opsGroup.WithBearerAuth()

	streamGroup := r.v1.Group("/workspaces/current", r.mw.auth, r.mw.workspaceQuery).WithTagInfo(okapi.GroupTag{
		Name:        "Messages",
		Description: "Web form definitions, received messages, operator replies, and spam filters for the active workspace.",
	})
	streamGroup.WithBearerAuth()

	return []okapi.RouteDefinition{
		{
			Method:      http.MethodPost,
			Path:        "/forms",
			Handler:     okapi.H(r.h.form.Create),
			Group:       opsGroup,
			Summary:     "Create a form",
			Description: "Creates a web form endpoint and returns its public key.",
			Request:     &handlers.CreateFormRequest{},
			Options: []okapi.RouteOption{
				okapi.DocResponse(201, &dto.Response[models.Form]{}),
				okapi.DocErrorResponse(409, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:   http.MethodGet,
			Path:     "/forms",
			Handler:  okapi.H(r.h.form.List),
			Group:    opsGroup,
			Summary:  "List forms",
			Request:  &handlers.ListRequest{},
			Response: &dto.PageableResponse[models.Form]{},
		},
		{
			Method:   http.MethodGet,
			Path:     "/forms/{id:int}",
			Handler:  okapi.H(r.h.form.Get),
			Group:    opsGroup,
			Summary:  "Get a form",
			Response: &dto.Response[models.Form]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Form ID"),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:   http.MethodPut,
			Path:     "/forms/{id:int}",
			Handler:  okapi.H(r.h.form.Update),
			Group:    opsGroup,
			Summary:  "Update a form",
			Request:  &handlers.UpdateFormRequest{},
			Response: &dto.Response[models.Form]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Form ID"),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:  http.MethodDelete,
			Path:    "/forms/{id:int}",
			Handler: okapi.H(r.h.form.Delete),
			Group:   opsGroup,
			Summary: "Delete a form",
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Form ID"),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodPost,
			Path:        "/forms/{id:int}/rotate-key",
			Handler:     okapi.H(r.h.form.RotateKey),
			Group:       opsGroup,
			Summary:     "Rotate a form public key",
			Description: "Issues a new public key. Existing embeds stop working immediately.",
			Response:    &dto.Response[models.Form]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Form ID"),
			},
		},
		{
			Method:      http.MethodGet,
			Path:        "/forms/{id:int}/snippet",
			Handler:     okapi.H(r.h.form.Snippet),
			Group:       opsGroup,
			Summary:     "Get embed code for a form",
			Description: "Returns paste-ready HTML and fetch() snippets wired to this form's endpoint.",
			Response:    &dto.Response[handlers.FormSnippetResponse]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Form ID"),
			},
		},
		{
			Method:   http.MethodGet,
			Path:     "/messages",
			Handler:  okapi.H(r.h.message.List),
			Group:    opsGroup,
			Summary:  "List messages",
			Request:  &handlers.MessageListRequest{},
			Response: &dto.PageableResponse[models.Message]{},
		},
		{
			Method:      http.MethodGet,
			Path:        "/messages/stats",
			Handler:     r.h.message.Stats,
			Group:       opsGroup,
			Summary:     "Message counters",
			Description: "Total, unread, and spam counts plus the number of forms in the workspace.",
			Response:    &dto.Response[handlers.MessageStatsResponse]{},
		},
		{
			Method:   http.MethodGet,
			Path:     "/messages/analytics",
			Handler:  okapi.H(r.h.message.Analytics),
			Group:    opsGroup,
			Summary:  "Message volume analytics",
			Request:  &handlers.MessageAnalyticsRequest{},
			Response: &dto.Response[handlers.MessageAnalyticsResponse]{},
		},
		{
			Method:   http.MethodGet,
			Path:     "/messages/{id}",
			Handler:  okapi.H(r.h.message.Get),
			Group:    opsGroup,
			Summary:  "Get a message",
			Response: &dto.Response[models.Message]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "string", "Message UUID"),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:  http.MethodDelete,
			Path:    "/messages/{id}",
			Handler: okapi.H(r.h.message.Delete),
			Group:   opsGroup,
			Summary: "Delete a message",
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "string", "Message UUID"),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodPost,
			Path:        "/messages/{id}/reply",
			Handler:     okapi.H(r.h.message.Reply),
			Group:       opsGroup,
			Summary:     "Reply to a message",
			Description: "Sends an operator reply to the sender through the workspace's normal email pipeline and records it on the thread.",
			Request:     &handlers.ReplyMessageRequest{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "string", "Message UUID"),
				okapi.DocResponse(201, &dto.Response[models.MessageReply]{}),
				okapi.DocErrorResponse(400, &dto.ErrorResponseBody{}),
				okapi.DocErrorResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:   http.MethodPut,
			Path:     "/messages/{id}/state",
			Handler:  okapi.H(r.h.message.UpdateState),
			Group:    opsGroup,
			Summary:  "Update message state",
			Request:  &handlers.UpdateMessageStateRequest{},
			Response: &dto.Response[models.Message]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "string", "Message UUID"),
			},
		},
		{
			Method:   http.MethodPut,
			Path:     "/messages/{id}/assign",
			Handler:  okapi.H(r.h.message.Assign),
			Group:    opsGroup,
			Summary:  "Assign a message",
			Request:  &handlers.AssignMessageRequest{},
			Response: &dto.Response[models.Message]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "string", "Message UUID"),
			},
		},
		{
			Method:      http.MethodPost,
			Path:        "/messages/{id}/spam",
			Handler:     okapi.H(r.h.message.MarkSpam),
			Group:       opsGroup,
			Summary:     "Mark a message as spam",
			Description: "Quarantines the message and optionally creates a filter from it.",
			Request:     &handlers.MarkSpamRequest{},
			Response:    &dto.Response[models.Message]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "string", "Message UUID"),
			},
		},
		{
			Method:   http.MethodPost,
			Path:     "/messages/{id}/not-spam",
			Handler:  okapi.H(r.h.message.MarkNotSpam),
			Group:    opsGroup,
			Summary:  "Clear the spam verdict on a message",
			Response: &dto.Response[models.Message]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "string", "Message UUID"),
			},
		},
		{
			Method:  http.MethodGet,
			Path:    "/messages/{id}/attachments/{idx:int}",
			Handler: okapi.H(r.h.message.DownloadAttachment),
			Group:   opsGroup,
			Summary: "Download a message attachment",
			Request: &handlers.MessageAttachmentRequest{},
		},
		{
			Method:  http.MethodGet,
			Path:    "/message-stream",
			Handler: r.h.message.Stream,
			Group:   streamGroup,
			Summary: "Server-sent events stream of message events",
			Options: []okapi.RouteOption{okapi.DocHide()},
		},
		{
			Method:  http.MethodPost,
			Path:    "/message-filters",
			Handler: okapi.H(r.h.messageFilter.Create),
			Group:   opsGroup,
			Summary: "Create a spam filter",
			Request: &handlers.CreateMessageFilterRequest{},
			Options: []okapi.RouteOption{
				okapi.DocResponse(201, &dto.Response[models.MessageFilter]{}),
				okapi.DocErrorResponse(400, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:   http.MethodGet,
			Path:     "/message-filters",
			Handler:  okapi.H(r.h.messageFilter.List),
			Group:    opsGroup,
			Summary:  "List spam filters",
			Request:  &handlers.ListRequest{},
			Response: &dto.PageableResponse[models.MessageFilter]{},
		},
		{
			Method:   http.MethodPut,
			Path:     "/message-filters/{id:int}",
			Handler:  okapi.H(r.h.messageFilter.Update),
			Group:    opsGroup,
			Summary:  "Update a spam filter",
			Request:  &handlers.UpdateMessageFilterRequest{},
			Response: &dto.Response[models.MessageFilter]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Filter ID"),
			},
		},
		{
			Method:  http.MethodDelete,
			Path:    "/message-filters/{id:int}",
			Handler: okapi.H(r.h.messageFilter.Delete),
			Group:   opsGroup,
			Summary: "Delete a spam filter",
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Filter ID"),
			},
		},
		{
			Method:      http.MethodPost,
			Path:        "/message-filters/test",
			Handler:     okapi.H(r.h.messageFilter.Test),
			Group:       opsGroup,
			Summary:     "Dry-run a spam filter",
			Description: "Runs a candidate pattern over recent messages and reports how many it would have matched.",
			Request:     &handlers.TestMessageFilterRequest{},
			Response:    &dto.Response[handlers.TestMessageFilterResponse]{},
		},
	}
}
