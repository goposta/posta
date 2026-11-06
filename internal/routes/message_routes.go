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
	"github.com/goposta/posta/internal/services/messages"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
)

func (r *Router) formIngestRoutes() []okapi.RouteDefinition {
	ingestGroup := r.app.Group("/api/v1/f").WithTagInfo(okapi.GroupTag{
		Name:        tagForms,
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
				okapi.DocResponse(403, &dto.ErrorResponseBody{}),
				okapi.DocResponse(429, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodGet,
			Path:        "/{key}/config",
			Handler:     r.h.formIngest.WidgetConfig,
			Group:       ingestGroup,
			Summary:     "Get the website widget config",
			Description: "What the website form widget needs before rendering: whether the form is usable from the calling origin, its honeypot field, and a nonce when the form requires one. Always answers 200; a form that cannot be used has enabled=false and a reason.",
			Response:    &dto.Response[handlers.WidgetConfig]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("key", "string", "Form public key"),
			},
		},
		{
			Method:  http.MethodGet,
			Path:    "/f/{key}",
			Handler: r.h.formIngest.HostedPage,
			Options: []okapi.RouteOption{okapi.DocHide()},
		},
		{
			Method:  http.MethodGet,
			Path:    "/posta-form.js",
			Handler: r.h.formIngest.FormScript,
			Group:   r.app.Group("/widget/v1"),
			Options: []okapi.RouteOption{okapi.DocHide()},
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
		Name:        tagMessages,
		Description: "Web form definitions, received messages, operator replies, and spam filters for the active workspace. Authenticated with a dashboard session or an API key bound to the workspace.",
	})
	opsGroup.WithBearerAuth()

	streamGroup := r.v1.Group("/workspaces/current", r.mw.auth, r.mw.workspaceQuery).WithTagInfo(okapi.GroupTag{
		Name:        tagMessages,
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
				okapi.DocResponse(409, &dto.ErrorResponseBody{}),
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
			Path:     pathFormByID,
			Handler:  okapi.H(r.h.form.Get),
			Group:    opsGroup,
			Summary:  "Get a form",
			Response: &dto.Response[models.Form]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Form ID"),
				okapi.DocResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:   http.MethodPut,
			Path:     pathFormByID,
			Handler:  okapi.H(r.h.form.Update),
			Group:    opsGroup,
			Summary:  "Update a form",
			Request:  &handlers.UpdateFormRequest{},
			Response: &dto.Response[models.Form]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Form ID"),
				okapi.DocResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:  http.MethodDelete,
			Path:    pathFormByID,
			Handler: okapi.H(r.h.form.Delete),
			Group:   opsGroup,
			Summary: "Delete a form",
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Form ID"),
				okapi.DocResponse(404, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodPost,
			Path:        "/forms/{id}/rotate-key",
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
			Path:        "/forms/{id}/snippet",
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
			Path:        "/form-submissions",
			Handler:     okapi.H(r.h.message.ListSubmissions),
			Group:       opsGroup,
			Summary:     "List a form's submissions",
			Description: "Pages through a form's responses, newest first. Filter answers with field.<key>=value, or field.<key>[gte]=n and field.<key>[lte]=n on number, scale, rating, date and time questions. Spam is left out unless status asks for it.",
			Request:     &handlers.SubmissionQueryRequest{},
			Response:    &dto.PageableResponse[models.Message]{},
		},
		{
			Method:      http.MethodGet,
			Path:        "/form-submissions/summary",
			Handler:     okapi.H(r.h.message.SubmissionSummary),
			Group:       opsGroup,
			Summary:     "Summarize a form's submissions",
			Description: "Per-question figures over the same filters: option counts, scale and rating histograms with means, number ranges, and the latest text answers. Personal-data answers are quoted only for workspace admins or admin-scoped keys.",
			Request:     &handlers.SubmissionQueryRequest{},
			Response:    &dto.Response[messages.SubmissionSummary]{},
		},
		{
			Method:      http.MethodGet,
			Path:        "/form-submissions/export",
			Handler:     okapi.H(r.h.message.ExportSubmissions),
			Group:       streamGroup,
			Summary:     "Export a form's submissions",
			Description: "Streams the filtered responses as CSV (one column per question, choices by label) or JSON (typed answers). Personal-data questions are included only for workspace admins or admin-scoped keys; X-Posta-Withheld-Fields counts the ones left out. Each export is audit-logged.",
			Request:     &handlers.SubmissionQueryRequest{},
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
				okapi.DocResponse(404, &dto.ErrorResponseBody{}),
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
				okapi.DocResponse(404, &dto.ErrorResponseBody{}),
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
				okapi.DocResponse(400, &dto.ErrorResponseBody{}),
				okapi.DocResponse(404, &dto.ErrorResponseBody{}),
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
			Method:      http.MethodPut,
			Path:        "/messages/{id}/tags",
			Handler:     okapi.H(r.h.message.UpdateTags),
			Group:       opsGroup,
			Summary:     "Add or remove message tags",
			Description: "Tags are lowercased and deduplicated. A tag in both add and remove is removed.",
			Request:     &handlers.UpdateMessageTagsRequest{},
			Response:    &dto.Response[models.Message]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "string", "Message UUID"),
				okapi.DocResponse(400, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:   http.MethodPut,
			Path:     "/messages/{id}/priority",
			Handler:  okapi.H(r.h.message.UpdatePriority),
			Group:    opsGroup,
			Summary:  "Set message priority",
			Request:  &handlers.UpdateMessagePriorityRequest{},
			Response: &dto.Response[models.Message]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "string", "Message UUID"),
			},
		},
		{
			Method:      http.MethodPut,
			Path:        "/messages/{id}/team",
			Handler:     okapi.H(r.h.message.UpdateTeam),
			Group:       opsGroup,
			Middlewares: r.licensed(),
			Summary:     "Route a message to a team",
			Request:     &handlers.UpdateMessageTeamRequest{},
			Response:    &dto.Response[models.Message]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "string", "Message UUID"),
			},
		},
		{
			Method:      http.MethodGet,
			Path:        "/message-tags",
			Handler:     r.h.message.Tags,
			Group:       opsGroup,
			Summary:     "List message tags in use",
			Description: "Tags on the workspace's recent messages, most used first, for suggestions and filters.",
			Response:    &dto.Response[[]repositories.TagCount]{},
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
			Path:    "/messages/{id}/attachments/{idx}",
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
				okapi.DocResponse(400, &dto.ErrorResponseBody{}),
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
			Path:     "/message-filters/{id}",
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
			Path:    "/message-filters/{id}",
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

		// Routing rules are form configuration, so admin scope like /forms; rules and teams need an EE license (402).
		{
			Method:      http.MethodGet,
			Path:        "/forms/{id}/rules",
			Handler:     okapi.H(r.h.formRule.List),
			Group:       opsGroup,
			Middlewares: r.licensed(),
			Summary:     "List a form's routing rules",
			Response:    &dto.Response[[]models.FormRule]{},
			Options:     []okapi.RouteOption{okapi.DocPathParam("id", "integer", "Form ID")},
		},
		{
			Method:      http.MethodPost,
			Path:        "/forms/{id}/rules",
			Handler:     okapi.H(r.h.formRule.Create),
			Group:       opsGroup,
			Middlewares: r.licensedToChange(),
			Summary:     "Create a routing rule",
			Description: "Appends a rule. Conditions test answers or built-in facts ($origin, $referer, $sender_email, $spam_score, $status); actions notify, assign, tag, prioritize or call a webhook. Every destination is one an admin chose: addresses typed here, or this workspace's members, teams and webhooks.",
			Request:     &handlers.CreateFormRuleRequest{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Form ID"),
				okapi.DocResponse(201, &dto.Response[models.FormRule]{}),
				okapi.DocResponse(400, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodPut,
			Path:        "/forms/{id}/rules/order",
			Handler:     okapi.H(r.h.formRule.Reorder),
			Group:       opsGroup,
			Middlewares: r.licensedToChange(),
			Summary:     "Reorder a form's routing rules",
			Request:     &handlers.ReorderFormRulesRequest{},
			Response:    &dto.Response[[]models.FormRule]{},
			Options:     []okapi.RouteOption{okapi.DocPathParam("id", "integer", "Form ID")},
		},
		{
			Method:      http.MethodPost,
			Path:        "/forms/{id}/rules/test",
			Handler:     okapi.H(r.h.formRule.Test),
			Group:       opsGroup,
			Middlewares: r.licensed(),
			Summary:     "Dry-run routing rules",
			Description: "Replays up to 200 recent submissions through the saved rules, or through draft rules sent in the body, and reports what they would have done. Nothing is sent or changed.",
			Request:     &handlers.TestFormRulesRequest{},
			Response:    &dto.Response[handlers.RuleTestResult]{},
			Options:     []okapi.RouteOption{okapi.DocPathParam("id", "integer", "Form ID")},
		},
		{
			Method:      http.MethodPut,
			Path:        "/forms/{id}/rules/{rule_id}",
			Handler:     okapi.H(r.h.formRule.Update),
			Group:       opsGroup,
			Middlewares: r.licensedToChange(),
			Summary:     "Update a routing rule",
			Request:     &handlers.UpdateFormRuleRequest{},
			Response:    &dto.Response[models.FormRule]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Form ID"),
				okapi.DocPathParam("rule_id", "integer", "Rule ID"),
			},
		},
		{
			Method:      http.MethodDelete,
			Path:        "/forms/{id}/rules/{rule_id}",
			Handler:     okapi.H(r.h.formRule.Delete),
			Group:       opsGroup,
			Middlewares: r.licensedToChange(),
			Summary:     "Delete a routing rule",
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Form ID"),
				okapi.DocPathParam("rule_id", "integer", "Rule ID"),
			},
		},

		// Teams are tenant administration (admin scope) but listed for every member.
		{
			Method:      http.MethodGet,
			Path:        "/teams",
			Handler:     r.h.team.List,
			Group:       opsGroup,
			Middlewares: r.licensed(),
			Summary:     "List teams",
			Response:    &dto.Response[[]models.Team]{},
		},
		{
			Method:      http.MethodPost,
			Path:        "/teams",
			Handler:     okapi.H(r.h.team.Create),
			Group:       opsGroup,
			Middlewares: r.licensedToChange(),
			Summary:     "Create a team",
			Request:     &handlers.CreateTeamRequest{},
			Options: []okapi.RouteOption{
				okapi.DocResponse(201, &dto.Response[models.Team]{}),
				okapi.DocResponse(409, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodGet,
			Path:        pathTeamByID,
			Handler:     okapi.H(r.h.team.Get),
			Group:       opsGroup,
			Middlewares: r.licensed(),
			Summary:     "Get a team",
			Description: "The team with its members, the routing rules that notify or assign it, and how many submissions it holds.",
			Request:     &handlers.TeamIDRequest{},
			Response:    &dto.Response[handlers.TeamDetail]{},
			Options:     []okapi.RouteOption{okapi.DocPathParam("id", "integer", "Team ID")},
		},
		{
			Method:      http.MethodPut,
			Path:        pathTeamByID,
			Handler:     okapi.H(r.h.team.Update),
			Group:       opsGroup,
			Middlewares: r.licensedToChange(),
			Summary:     "Update a team and its members",
			Request:     &handlers.UpdateTeamRequest{},
			Response:    &dto.Response[models.Team]{},
			Options:     []okapi.RouteOption{okapi.DocPathParam("id", "integer", "Team ID")},
		},
		{
			Method:      http.MethodDelete,
			Path:        pathTeamByID,
			Handler:     okapi.H(r.h.team.Delete),
			Group:       opsGroup,
			Middlewares: r.licensedToChange(),
			Summary:     "Delete a team",
			Description: "Refused while a routing rule notifies or assigns the team. Submissions routed to it keep no team.",
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Team ID"),
				okapi.DocResponse(409, &dto.ErrorResponseBody{}),
			},
		},
	}
}
