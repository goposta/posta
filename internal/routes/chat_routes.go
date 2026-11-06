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
	"github.com/goposta/posta/internal/middlewares"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/chat"
	"github.com/goposta/posta/internal/services/messages"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
)

// Each chat path's OPTIONS route comes first so config.ChatPreflight answers before okapi's global CORS handler.
func (r *Router) chatPublicRoutes() []okapi.RouteDefinition {
	group := r.app.Group("/api/v1/chat").WithTagInfo(okapi.GroupTag{
		Name:        tagChat,
		Description: "Public live chat API used by the chat widget. No session: a visitor authenticates with the bearer token returned when they first write, and a widget is usable only from its allowed origins.",
	})
	visitorAuth := okapi.DocHeader("Authorization", "string", "Bearer pcv_… visitor token", true)
	keyParam := okapi.DocPathParam("key", "string", "Chat widget public key")
	idParam := okapi.DocPathParam("id", "string", "Conversation ID")
	msgParam := okapi.DocPathParam("msg", "string", "Message ID")
	idxParam := okapi.DocPathParam("idx", "integer", "Attachment index")

	preflight := func(path string) okapi.RouteDefinition {
		return okapi.RouteDefinition{
			Method: http.MethodOptions, Path: path, Handler: r.h.chatVisitor.Preflight,
			Group: group, Options: []okapi.RouteOption{okapi.DocHide()},
		}
	}

	return []okapi.RouteDefinition{
		preflight("/{key}/config"),
		preflight("/{key}/nonce"),
		preflight(pathChatConversations),
		preflight(pathChatMessages),
		preflight("/{key}/conversations/{id}/stream"),
		preflight("/{key}/conversations/{id}/typing"),
		preflight("/{key}/conversations/{id}/read"),
		preflight("/{key}/conversations/{id}/attachments"),
		preflight("/{key}/conversations/{id}/messages/{msg}/attachments/{idx}"),
		preflight("/{key}/identify"),
		preflight("/{key}/attributes"),
		{
			Method:      http.MethodGet,
			Path:        "/{key}/config",
			Handler:     okapi.H(r.h.chatVisitor.Config),
			Group:       group,
			Summary:     "Get the chat widget config",
			Description: "Appearance, pre-chat requirements and a nonce for starting a conversation. Always answers 200; a widget that cannot be used from the calling origin has enabled=false and a reason.",
			Response:    &dto.Response[handlers.ChatWidgetConfig]{},
			Options:     []okapi.RouteOption{keyParam},
		},
		{
			Method:   http.MethodGet,
			Path:     "/{key}/nonce",
			Handler:  okapi.H(r.h.chatVisitor.Nonce),
			Group:    group,
			Summary:  "Issue a conversation nonce",
			Response: &dto.Response[messages.Nonce]{},
			Options:  []okapi.RouteOption{keyParam},
		},
		{
			Method:      http.MethodPost,
			Path:        pathChatConversations,
			Handler:     okapi.H(r.h.chatVisitor.Start),
			Group:       group,
			Summary:     "Start a conversation",
			Description: "Stores the visitor's first message. A new visitor gets a visitor_token to send as a bearer token from then on; a returning visitor sends theirs. A message that spam scanning rejects answers 202 with status received and starts nothing.",
			Request:     &handlers.ChatStartRequest{},
			Options: []okapi.RouteOption{
				keyParam,
				okapi.DocResponse(201, &dto.Response[handlers.ChatStartResponse]{}),
				okapi.DocResponse(202, &dto.Response[handlers.ChatStartResponse]{}),
				okapi.DocResponse(429, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:   http.MethodGet,
			Path:     pathChatConversations,
			Handler:  okapi.H(r.h.chatVisitor.Conversations),
			Group:    group,
			Summary:  "List a visitor's conversations",
			Response: &dto.Response[[]chat.VisitorConversation]{},
			Options:  []okapi.RouteOption{keyParam, visitorAuth},
		},
		{
			Method:      http.MethodGet,
			Path:        pathChatMessages,
			Handler:     okapi.H(r.h.chatVisitor.Messages),
			Group:       group,
			Summary:     "List messages as a visitor",
			Description: "after returns messages with a higher sequence number, for catching up or polling when the stream is unavailable; before pages back through history.",
			Request:     &handlers.ChatVisitorMessagesRequest{},
			Response:    &dto.Response[[]chat.VisitorMessage]{},
			Options:     []okapi.RouteOption{keyParam, idParam, visitorAuth},
		},
		{
			Method:  http.MethodPost,
			Path:    pathChatMessages,
			Handler: okapi.H(r.h.chatVisitor.PostMessage),
			Group:   group,
			Summary: "Send a message as a visitor",
			Request: &handlers.ChatVisitorPostRequest{},
			Options: []okapi.RouteOption{
				keyParam, idParam, visitorAuth,
				okapi.DocResponse(201, &dto.Response[chat.VisitorMessage]{}),
				okapi.DocResponse(429, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodGet,
			Path:        "/{key}/conversations/{id}/stream",
			Handler:     r.h.chatVisitor.Stream,
			Group:       group,
			Summary:     "Stream a conversation as a visitor",
			Description: "Server-sent events: chat.message and chat.conversation. Resumes after Last-Event-ID (or ?after=). Answers 503 with Retry-After when this server holds too many streams; poll the messages endpoint instead.",
			Options:     []okapi.RouteOption{keyParam, idParam, visitorAuth},
		},
		{
			Method:      http.MethodPost,
			Path:        "/{key}/conversations/{id}/typing",
			Handler:     okapi.H(r.h.chatVisitor.Typing),
			Group:       group,
			Summary:     "Say the visitor is typing",
			Description: "Shows agents a typing indicator. At most one every 3 seconds is passed on; nothing is stored.",
			Options:     []okapi.RouteOption{keyParam, idParam, visitorAuth, okapi.DocResponse(204, nil)},
		},
		{
			Method:   http.MethodPost,
			Path:     "/{key}/conversations/{id}/read",
			Handler:  okapi.H(r.h.chatVisitor.Read),
			Group:    group,
			Summary:  "Mark messages read as a visitor",
			Request:  &handlers.ChatVisitorReadRequest{},
			Response: &dto.Response[handlers.ChatReadResponse]{},
			Options:  []okapi.RouteOption{keyParam, idParam, visitorAuth},
		},
		{
			Method:      http.MethodPost,
			Path:        "/{key}/identify",
			Handler:     okapi.H(r.h.chatVisitor.Identify),
			Group:       group,
			Summary:     "Identify a visitor",
			Description: "Attaches a signed identity from the customer's server. A valid signature verifies the visitor's email and joins their history across devices; an invalid one answers verified=false and changes nothing.",
			Request:     &handlers.ChatIdentifyRequest{},
			Response:    &dto.Response[handlers.ChatIdentifyResponse]{},
			Options: []okapi.RouteOption{keyParam,
				okapi.DocHeader("Authorization", "string", "Bearer pcv_… visitor token; without one, a verified identity creates a visitor and returns its token", false)},
		},
		{
			Method:  http.MethodPost,
			Path:    "/{key}/attributes",
			Handler: okapi.H(r.h.chatVisitor.Attributes),
			Group:   group,
			Summary: "Set a visitor's attributes",
			Request: &handlers.ChatAttributesRequest{},
			Options: []okapi.RouteOption{keyParam, visitorAuth, okapi.DocResponse(204, nil)},
		},
		{
			Method:      http.MethodPost,
			Path:        "/{key}/conversations/{id}/attachments",
			Handler:     r.h.chatVisitor.Upload,
			Group:       group,
			Summary:     "Send a file as a visitor",
			Description: "multipart/form-data with a file part (PNG, JPEG, GIF, WebP or PDF, up to 5 MB) and optional caption and client_msg_id parts. The type is read from the file itself.",
			Options: []okapi.RouteOption{keyParam, idParam, visitorAuth,
				okapi.DocResponse(201, &dto.Response[chat.VisitorMessage]{}),
				okapi.DocResponse(413, &dto.ErrorResponseBody{})},
		},
		{
			Method:  http.MethodGet,
			Path:    "/{key}/conversations/{id}/messages/{msg}/attachments/{idx}",
			Handler: okapi.H(r.h.chatVisitor.Attachment),
			Group:   group,
			Summary: "Download a file as a visitor",
			Options: []okapi.RouteOption{keyParam, idParam, visitorAuth, msgParam, idxParam},
		},
		{
			Method:  http.MethodGet,
			Path:    "/posta-chat.js",
			Handler: r.h.chatVisitor.ChatScript,
			Group:   r.app.Group("/widget/v1"),
			Options: []okapi.RouteOption{okapi.DocHide()},
		},
	}
}

func (r *Router) chatWorkspaceRoutes() []okapi.RouteDefinition {
	opsGroup := r.v1.Group("/workspaces/current", r.mw.auth, r.mw.workspace).WithTagInfo(okapi.GroupTag{
		Name:        tagChat,
		Description: "Live chat widgets and conversations for the active workspace. Authenticated with a dashboard session or an API key bound to the workspace.",
	})
	opsGroup.WithBearerAuth()
	streamGroup := r.v1.Group("/workspaces/current", r.mw.auth, r.mw.workspaceQuery).WithTagInfo(okapi.GroupTag{
		Name: tagChat,
	})
	streamGroup.WithBearerAuth()

	widgetID := okapi.DocPathParam("id", "integer", "Chat widget ID")
	convID := okapi.DocPathParam("id", "string", "Conversation ID")
	notFound := okapi.DocResponse(404, &dto.ErrorResponseBody{})

	return []okapi.RouteDefinition{
		{
			Method:  http.MethodPost,
			Path:    "/chat-widgets",
			Handler: okapi.H(r.h.chatWidget.Create),
			Group:   opsGroup,
			Summary: "Create a chat widget",
			Request: &handlers.CreateChatWidgetRequest{},
			Options: []okapi.RouteOption{
				okapi.DocResponse(201, &dto.Response[models.ChatWidget]{}),
				okapi.DocResponse(409, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:   http.MethodGet,
			Path:     "/chat-widgets",
			Handler:  okapi.H(r.h.chatWidget.List),
			Group:    opsGroup,
			Summary:  "List chat widgets",
			Request:  &handlers.ListRequest{},
			Response: &dto.PageableResponse[models.ChatWidget]{},
		},
		{
			Method:   http.MethodGet,
			Path:     pathChatWidgetByID,
			Handler:  okapi.H(r.h.chatWidget.Get),
			Group:    opsGroup,
			Summary:  "Get a chat widget",
			Response: &dto.Response[models.ChatWidget]{},
			Options:  []okapi.RouteOption{widgetID, notFound},
		},
		{
			Method:   http.MethodPut,
			Path:     pathChatWidgetByID,
			Handler:  okapi.H(r.h.chatWidget.Update),
			Group:    opsGroup,
			Summary:  "Update a chat widget",
			Request:  &handlers.UpdateChatWidgetRequest{},
			Response: &dto.Response[models.ChatWidget]{},
			Options:  []okapi.RouteOption{widgetID, notFound},
		},
		{
			Method:  http.MethodDelete,
			Path:    pathChatWidgetByID,
			Handler: okapi.H(r.h.chatWidget.Delete),
			Group:   opsGroup,
			Summary: "Delete a chat widget",
			Options: []okapi.RouteOption{widgetID, notFound},
		},
		{
			Method:      http.MethodPost,
			Path:        "/chat-widgets/{id}/rotate-key",
			Handler:     okapi.H(r.h.chatWidget.RotateKey),
			Group:       opsGroup,
			Summary:     "Rotate a chat widget's public key",
			Description: "Issues a new public key. Existing embeds stop working immediately.",
			Response:    &dto.Response[models.ChatWidget]{},
			Options:     []okapi.RouteOption{widgetID},
		},
		{
			Method:   http.MethodGet,
			Path:     "/chat-widgets/{id}/snippet",
			Handler:  okapi.H(r.h.chatWidget.Snippet),
			Group:    opsGroup,
			Summary:  "Get the embed code for a chat widget",
			Response: &dto.Response[handlers.ChatWidgetSnippetResponse]{},
			Options:  []okapi.RouteOption{widgetID},
		},
		{
			Method:   http.MethodGet,
			Path:     "/conversations",
			Handler:  okapi.H(r.h.chatConversation.List),
			Group:    opsGroup,
			Summary:  "List chat conversations",
			Request:  &handlers.ChatConversationListRequest{},
			Response: &dto.PageableResponse[models.ChatConversation]{},
		},
		{
			Method:   http.MethodGet,
			Path:     pathConversationByID,
			Handler:  okapi.H(r.h.chatConversation.Get),
			Group:    opsGroup,
			Summary:  "Get a chat conversation",
			Response: &dto.Response[models.ChatConversation]{},
			Options:  []okapi.RouteOption{convID, notFound},
		},
		{
			Method:      http.MethodGet,
			Path:        "/conversations/{id}/messages",
			Handler:     okapi.H(r.h.chatConversation.Messages),
			Group:       opsGroup,
			Summary:     "List a conversation's messages",
			Description: "after returns newer messages by sequence number, before pages back through history; with neither, the newest page.",
			Request:     &handlers.ChatMessagesRequest{},
			Response:    &dto.Response[[]models.ChatMessage]{},
			Options:     []okapi.RouteOption{convID, notFound},
		},
		{
			Method:      http.MethodPost,
			Path:        "/conversations/{id}/messages",
			Handler:     okapi.H(r.h.chatConversation.PostMessage),
			Group:       opsGroup,
			Summary:     "Reply in a conversation",
			Description: "Editor role or higher. Replying to an unassigned conversation assigns it to the caller.",
			Request:     &handlers.ChatPostMessageRequest{},
			Options: []okapi.RouteOption{
				convID, notFound,
				okapi.DocResponse(201, &dto.Response[models.ChatMessage]{}),
			},
		},
		{
			Method:   http.MethodPut,
			Path:     "/conversations/{id}/state",
			Handler:  okapi.H(r.h.chatConversation.SetState),
			Group:    opsGroup,
			Summary:  "Change a conversation's state",
			Request:  &handlers.ChatStateRequest{},
			Response: &dto.Response[models.ChatConversation]{},
			Options:  []okapi.RouteOption{convID, notFound},
		},
		{
			Method:   http.MethodPut,
			Path:     "/conversations/{id}/assign",
			Handler:  okapi.H(r.h.chatConversation.Assign),
			Group:    opsGroup,
			Summary:  "Assign a conversation",
			Request:  &handlers.ChatAssignRequest{},
			Response: &dto.Response[models.ChatConversation]{},
			Options:  []okapi.RouteOption{convID, notFound},
		},
		{
			Method:      http.MethodPost,
			Path:        "/conversations/{id}/typing",
			Handler:     okapi.H(r.h.chatConversation.Typing),
			Group:       opsGroup,
			Summary:     "Say an agent is typing",
			Description: "Shows the visitor a typing indicator. Throttled; nothing is stored.",
			Options:     []okapi.RouteOption{convID, notFound, okapi.DocResponse(204, nil)},
		},
		{
			Method:   http.MethodPost,
			Path:     "/conversations/{id}/read",
			Handler:  okapi.H(r.h.chatConversation.Read),
			Group:    opsGroup,
			Summary:  "Mark messages read as an agent",
			Request:  &handlers.ChatReadRequest{},
			Response: &dto.Response[handlers.ChatReadResponse]{},
			Options:  []okapi.RouteOption{convID, notFound},
		},
		{
			Method:      http.MethodPut,
			Path:        "/chat/presence",
			Handler:     okapi.H(r.h.chatConversation.SetPresence),
			Group:       opsGroup,
			Summary:     "Set your chat presence",
			Description: "Heartbeat: send every 30 seconds while available. Without one for 75 seconds you read as offline. A widget is online when an agent is online and it is within its business hours.",
			Request:     &handlers.ChatPresenceRequest{},
			Options:     []okapi.RouteOption{okapi.DocResponse(204, nil)},
		},
		{
			Method:   http.MethodGet,
			Path:     "/chat/presence",
			Handler:  r.h.chatConversation.Presence,
			Group:    opsGroup,
			Summary:  "List agents online or away",
			Response: &dto.Response[[]handlers.ChatAgentPresence]{},
		},
		{
			Method:   http.MethodGet,
			Path:     "/conversations/{id}/visitor",
			Handler:  okapi.H(r.h.chatConversation.VisitorContext),
			Group:    opsGroup,
			Summary:  "Get a conversation's visitor, history and contact",
			Response: &dto.Response[handlers.ChatVisitorContext]{},
			Options:  []okapi.RouteOption{convID, notFound},
		},
		{
			Method:      http.MethodDelete,
			Path:        pathConversationByID,
			Handler:     okapi.H(r.h.chatConversation.Delete),
			Group:       opsGroup,
			Middlewares: []okapi.Middleware{middlewares.RequireWorkspaceRole(models.WorkspaceRoleAdmin)},
			Summary:     "Erase a conversation",
			Description: "Owners and admins. Deletes the conversation now, with its messages and files, and the visitor if it was their last conversation. Cannot be undone.",
			Options:     []okapi.RouteOption{convID, notFound, okapi.DocResponse(204, nil)},
		},
		{
			Method:      http.MethodPost,
			Path:        "/conversations/{id}/ban",
			Handler:     okapi.H(r.h.chatConversation.Ban),
			Group:       opsGroup,
			Summary:     "Ban a conversation's visitor",
			Description: "Marks the conversation spam, silently drops the visitor's further messages, and blocks their IP from starting chats in the workspace for 24 hours.",
			Options:     []okapi.RouteOption{convID, notFound, okapi.DocResponse(204, nil)},
		},
		{
			Method:  http.MethodDelete,
			Path:    "/conversations/{id}/ban",
			Handler: okapi.H(r.h.chatConversation.Unban),
			Group:   opsGroup,
			Summary: "Lift a visitor's ban",
			Options: []okapi.RouteOption{convID, notFound, okapi.DocResponse(204, nil)},
		},
		{
			Method:      http.MethodPost,
			Path:        "/conversations/{id}/attachments",
			Handler:     r.h.chatConversation.Upload,
			Group:       opsGroup,
			Summary:     "Send a file in a conversation",
			Description: "multipart/form-data with a file part (PNG, JPEG, GIF, WebP or PDF, up to 5 MB) and optional caption and client_msg_id parts.",
			Options: []okapi.RouteOption{convID, notFound,
				okapi.DocResponse(201, &dto.Response[models.ChatMessage]{}),
				okapi.DocResponse(413, &dto.ErrorResponseBody{})},
		},
		{
			Method:  http.MethodGet,
			Path:    "/conversations/{id}/messages/{msg}/attachments/{idx}",
			Handler: okapi.H(r.h.chatConversation.Attachment),
			Group:   opsGroup,
			Summary: "Download a file from a conversation",
			Options: []okapi.RouteOption{convID, notFound,
				okapi.DocPathParam("msg", "string", "Message ID"),
				okapi.DocPathParam("idx", "integer", "Attachment index")},
		},
		{
			Method:  http.MethodGet,
			Path:    "/chat/analytics",
			Handler: okapi.H(r.h.chatConversation.Analytics),
			Group:   opsGroup,
			// Posta Enterprise. Read-only, so a degraded license still shows it.
			Middlewares: r.licensed(),
			Summary:     "Chat analytics",
			Description: "Posta Enterprise: answers 402 without a license.",
			Request:     &handlers.ChatAnalyticsRequest{},
			Response:    &dto.Response[repositories.ChatAnalytics]{},
		},
		{
			Method:   http.MethodGet,
			Path:     "/chat-snippets",
			Handler:  r.h.chatSnippet.List,
			Group:    opsGroup,
			Summary:  "List canned replies",
			Response: &dto.Response[[]models.ChatSnippet]{},
		},
		{
			Method:  http.MethodPost,
			Path:    "/chat-snippets",
			Handler: okapi.H(r.h.chatSnippet.Create),
			Group:   opsGroup,
			Summary: "Create a canned reply",
			Request: &handlers.CreateChatSnippetRequest{},
			Options: []okapi.RouteOption{okapi.DocResponse(201, &dto.Response[models.ChatSnippet]{}), okapi.DocResponse(409, &dto.ErrorResponseBody{})},
		},
		{
			Method:   http.MethodPut,
			Path:     "/chat-snippets/{id}",
			Handler:  okapi.H(r.h.chatSnippet.Update),
			Group:    opsGroup,
			Summary:  "Update a canned reply",
			Request:  &handlers.UpdateChatSnippetRequest{},
			Response: &dto.Response[models.ChatSnippet]{},
		},
		{
			Method:  http.MethodDelete,
			Path:    "/chat-snippets/{id}",
			Handler: okapi.H(r.h.chatSnippet.Delete),
			Group:   opsGroup,
			Summary: "Delete a canned reply",
			Options: []okapi.RouteOption{okapi.DocPathParam("id", "integer", "Canned reply ID")},
		},
		{
			Method:      http.MethodPost,
			Path:        "/chat-widgets/{id}/rotate-identity-secret",
			Handler:     okapi.H(r.h.chatWidget.RotateIdentitySecret),
			Group:       opsGroup,
			Summary:     "Rotate a chat widget's identity secret",
			Description: "Returns the new secret once. Identities signed with the old secret stop verifying; those visitors carry on anonymously.",
			Response:    &dto.Response[handlers.ChatIdentitySecretResponse]{},
			Options:     []okapi.RouteOption{widgetID},
		},
		{
			Method:   http.MethodGet,
			Path:     "/chat/summary",
			Handler:  r.h.chatConversation.Summary,
			Group:    opsGroup,
			Summary:  "Count conversations awaiting a reply",
			Response: &dto.Response[handlers.ChatSummary]{},
		},
		{
			Method:  http.MethodGet,
			Path:    "/chat-stream",
			Handler: r.h.chatConversation.Stream,
			Group:   streamGroup,
			Summary: "Server-sent events stream of chat events",
			Options: []okapi.RouteOption{okapi.DocHide()},
		},
	}
}

func (r *Router) unifiedInboxRoutes() []okapi.RouteDefinition {
	opsGroup := r.v1.Group("/workspaces/current", r.mw.auth, r.mw.workspace).WithTagInfo(okapi.GroupTag{
		Name:        tagMessages,
		Description: "Web form definitions, received messages, operator replies, and spam filters for the active workspace.",
	})
	opsGroup.WithBearerAuth()
	return []okapi.RouteDefinition{{
		Method:      http.MethodGet,
		Path:        "/inbox",
		Handler:     okapi.H(r.h.unifiedInbox.List),
		Group:       opsGroup,
		Summary:     "List the unified inbox",
		Description: "Form messages, inbound emails not already part of a thread, and live chats in one list, newest activity first. Each row names its source and ID; open it with that source's own endpoint.",
		Request:     &handlers.UnifiedInboxRequest{},
		Response:    &dto.PageableResponse[repositories.InboxItem]{},
	}}
}
