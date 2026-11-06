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

package handlers

import (
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/audit"
	"github.com/goposta/posta/internal/services/chat"
	"github.com/goposta/posta/internal/services/realtime"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
)

// ChatConversationHandler is the agents' side of live chat; ChatVisitorHandler serves the widget.
type ChatConversationHandler struct {
	audit      *audit.Logger
	svc        *chat.Service
	repo       *repositories.ChatRepository
	users      *repositories.UserRepository
	workspaces *repositories.WorkspaceRepository
}

func NewChatConversationHandler(svc *chat.Service, users *repositories.UserRepository, workspaces *repositories.WorkspaceRepository) *ChatConversationHandler {
	return &ChatConversationHandler{svc: svc, repo: svc.Repo(), users: users, workspaces: workspaces}
}

type ChatConversationListRequest struct {
	Page        int    `query:"page" default:"0"`
	Size        int    `query:"size" default:"30"`
	State       string `query:"state" enum:"open,pending,closed,spam"`
	Assigned    string `query:"assigned" enum:"me,unassigned,any" doc:"Whose conversations: me, unassigned, or any (default)"`
	WidgetID    int    `query:"widget_id"`
	Q           string `query:"q" doc:"Search visitor names and emails"`
	Quarantined bool   `query:"quarantined" doc:"Include conversations whose first message was quarantined"`
}

type ChatConversationRequest struct {
	ID string `param:"id"`
}

type ChatMessagesRequest struct {
	ID     string `param:"id"`
	After  int64  `query:"after" doc:"Only messages with a sequence number above this"`
	Before int64  `query:"before" doc:"Only messages with a sequence number below this"`
	Limit  int    `query:"limit" default:"50"`
}

type ChatPostMessageRequest struct {
	ID   string `param:"id"`
	Body struct {
		Body        string `json:"body" required:"true"`
		ClientMsgID string `json:"client_msg_id" doc:"Client-generated ID that makes a retried send idempotent"`
	} `json:"body"`
}

type ChatStateRequest struct {
	ID   string `param:"id"`
	Body struct {
		State string `json:"state" required:"true" enum:"open,pending,closed,spam"`
	} `json:"body"`
}

type ChatAssignRequest struct {
	ID   string `param:"id"`
	Body struct {
		UserID *uint `json:"user_id" doc:"An editor or above in this workspace, or null to unassign"`
	} `json:"body"`
}

type ChatPresenceRequest struct {
	Body struct {
		Status string `json:"status" required:"true" enum:"online,away,offline"`
	} `json:"body"`
}

type ChatAgentPresence struct {
	UserID uint   `json:"user_id"`
	Status string `json:"status" enum:"online,away"`
}

type ChatReadRequest struct {
	ID   string `param:"id"`
	Body struct {
		Seq int64 `json:"seq" doc:"Newest message the agent has seen"`
	} `json:"body"`
}

// SetAudit records bans in the audit log.
func (h *ChatConversationHandler) SetAudit(a *audit.Logger) { h.audit = a }

type ChatVisitorContext struct {
	Visitor *models.ChatVisitor       `json:"visitor"`
	History []models.ChatConversation `json:"history" doc:"The visitor's other conversations; for a verified visitor, also those from their other devices"`
	Contact *ChatContactRef           `json:"contact,omitempty" doc:"The workspace contact with the visitor's verified email"`
}

type ChatContactRef struct {
	ID    uint   `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type ChatAttachmentAgentRequest struct {
	ID  string `param:"id"`
	Msg string `param:"msg"`
	Idx int    `param:"idx"`
}

type ChatAnalyticsRequest struct {
	From     string `query:"from" doc:"RFC 3339 or YYYY-MM-DD; default 30 days ago"`
	To       string `query:"to" doc:"RFC 3339 or YYYY-MM-DD, exclusive; default now"`
	WidgetID int    `query:"widget_id"`
}

type ChatSummary struct {
	AwaitingReply int64 `json:"awaiting_reply" doc:"Open conversations whose last message is the visitor's"`
}

func (h *ChatConversationHandler) List(c *okapi.Context, req *ChatConversationListRequest) error {
	ws, err := workspaceOf(c)
	if err != nil {
		return c.AbortBadRequest(err.Error())
	}
	page, size := req.Page, req.Size
	if size <= 0 || size > 100 {
		size = 30
	}
	assigned := req.Assigned
	if assigned == "any" {
		assigned = ""
	}
	items, total, err := h.repo.ListConversations(ws, repositories.ChatConversationFilter{
		State:       req.State,
		Assigned:    assigned,
		UserID:      uint(c.GetInt("user_id")),
		WidgetID:    uint(max(req.WidgetID, 0)),
		Q:           req.Q,
		Quarantined: req.Quarantined,
	}, size, page*size)
	if err != nil {
		return c.AbortInternalServerError("failed to list conversations")
	}
	return paginated(c, items, total, page, size)
}

func (h *ChatConversationHandler) Summary(c *okapi.Context) error {
	ws, err := workspaceOf(c)
	if err != nil {
		return c.AbortBadRequest(err.Error())
	}
	n, err := h.repo.CountAwaitingReply(ws)
	if err != nil {
		return c.AbortInternalServerError("failed to count conversations")
	}
	return ok(c, ChatSummary{AwaitingReply: n})
}

func (h *ChatConversationHandler) Get(c *okapi.Context, req *ChatConversationRequest) error {
	conv, err := h.find(c, req.ID)
	if err != nil {
		return c.AbortNotFound("conversation not found")
	}
	h.svc.Describe(c.Request().Context(), conv)
	return ok(c, conv)
}

// SetPresence is a heartbeat sent every 30 seconds; an agent silent for 75 seconds reads as offline.
func (h *ChatConversationHandler) SetPresence(c *okapi.Context, req *ChatPresenceRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("only editors and above answer chats", err)
	}
	ws, err := workspaceOf(c)
	if err != nil {
		return c.AbortBadRequest(err.Error())
	}
	switch req.Body.Status {
	case chat.StatusOnline, chat.StatusAway, chat.StatusOffline:
	default:
		return c.AbortBadRequest("status must be online, away or offline")
	}
	if err := h.svc.SetPresence(c.Request().Context(), ws, uint(c.GetInt("user_id")), req.Body.Status); err != nil {
		if errors.Is(err, chat.ErrAgentLimit) {
			return c.AbortForbidden(err.Error(), err)
		}
		return c.AbortInternalServerError("failed to record presence")
	}
	return noContent(c)
}

func (h *ChatConversationHandler) Presence(c *okapi.Context) error {
	ws, err := workspaceOf(c)
	if err != nil {
		return c.AbortBadRequest(err.Error())
	}
	list, err := h.svc.Presence().List(c.Request().Context(), ws)
	if err != nil {
		return c.AbortInternalServerError("failed to read presence")
	}
	out := make([]ChatAgentPresence, 0, len(list))
	for id, status := range list {
		out = append(out, ChatAgentPresence{UserID: id, Status: status})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UserID < out[j].UserID })
	return ok(c, out)
}

// Typing is throttled and stores nothing.
func (h *ChatConversationHandler) Typing(c *okapi.Context, req *ChatConversationRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	conv, err := h.find(c, req.ID)
	if err != nil {
		return c.AbortNotFound("conversation not found")
	}
	name := ""
	if w, err := h.repo.FindWidgetByID(conv.WidgetID); err == nil && w.ShowAgentNames {
		if u, err := h.users.FindByID(uint(c.GetInt("user_id"))); err == nil {
			name = u.Name
		}
	}
	h.svc.Typing(c.Request().Context(), conv, false, name)
	return noContent(c)
}

func (h *ChatConversationHandler) Read(c *okapi.Context, req *ChatReadRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	conv, err := h.find(c, req.ID)
	if err != nil {
		return c.AbortNotFound("conversation not found")
	}
	seq, err := h.svc.MarkRead(c.Request().Context(), conv, false, req.Body.Seq)
	if err != nil {
		return c.AbortInternalServerError("failed to record read")
	}
	return ok(c, ChatReadResponse{Seq: seq})
}

func (h *ChatConversationHandler) Messages(c *okapi.Context, req *ChatMessagesRequest) error {
	conv, err := h.find(c, req.ID)
	if err != nil {
		return c.AbortNotFound("conversation not found")
	}
	items, err := pageMessages(h.repo, conv, req.After, req.Before, req.Limit)
	if err != nil {
		return c.AbortInternalServerError("failed to load messages")
	}
	return ok(c, items)
}

func (h *ChatConversationHandler) PostMessage(c *okapi.Context, req *ChatPostMessageRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	conv, err := h.find(c, req.ID)
	if err != nil {
		return c.AbortNotFound("conversation not found")
	}
	agent := chat.Agent{ID: uint(c.GetInt("user_id"))}
	if u, err := h.users.FindByID(agent.ID); err == nil {
		agent.Name = u.Name
	}
	msg, err := h.svc.PostAgentMessage(c.Request().Context(), conv, agent, req.Body.Body, req.Body.ClientMsgID)
	if err != nil {
		return abortChat(c, err)
	}
	return created(c, msg)
}

func (h *ChatConversationHandler) SetState(c *okapi.Context, req *ChatStateRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	state := models.ChatState(req.Body.State)
	if !models.ValidChatState(state) {
		return c.AbortBadRequest("state must be open, pending, closed or spam")
	}
	conv, err := h.find(c, req.ID)
	if err != nil {
		return c.AbortNotFound("conversation not found")
	}
	if err := h.svc.SetState(c.Request().Context(), conv, state); err != nil {
		return c.AbortInternalServerError("failed to update conversation")
	}
	return ok(c, conv)
}

func (h *ChatConversationHandler) Assign(c *okapi.Context, req *ChatAssignRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	conv, err := h.find(c, req.ID)
	if err != nil {
		return c.AbortNotFound("conversation not found")
	}
	var assignee *models.ActorRef
	if id := req.Body.UserID; id != nil {
		member, err := h.workspaces.FindMember(*conv.WorkspaceID, *id)
		if err != nil || !member.Role.CanEdit() {
			return c.AbortBadRequest("user_id must be an editor or above in this workspace")
		}
		assignee = &models.ActorRef{ID: *id}
		if u, err := h.users.FindByID(*id); err == nil {
			assignee.Name = u.Name
		}
	}
	if err := h.svc.Assign(c.Request().Context(), conv, req.Body.UserID); err != nil {
		return c.AbortInternalServerError("failed to assign conversation")
	}
	conv.AssignedTo = assignee
	return ok(c, conv)
}

// VisitorContext matches only a verified email to a contact: a typed one could be anyone's.
func (h *ChatConversationHandler) VisitorContext(c *okapi.Context, req *ChatConversationRequest) error {
	conv, err := h.find(c, req.ID)
	if err != nil {
		return c.AbortNotFound("conversation not found")
	}
	history, err := h.repo.VisitorHistory(conv, 20)
	if err != nil {
		return c.AbortInternalServerError("failed to load history")
	}
	if history == nil {
		history = []models.ChatConversation{}
	}
	out := ChatVisitorContext{Visitor: conv.Visitor, History: history}
	if v := conv.Visitor; v != nil && v.EmailVerified && v.Email != "" {
		if contact, err := h.repo.FindContactByEmail(*conv.WorkspaceID, v.Email); err == nil {
			out.Contact = &ChatContactRef{ID: contact.ID, Email: contact.Email, Name: contact.Name}
		}
	}
	return ok(c, out)
}

// Delete erases now, for a data subject's request, including files and the visitor if it was their last conversation.
func (h *ChatConversationHandler) Delete(c *okapi.Context, req *ChatConversationRequest) error {
	conv, err := h.find(c, req.ID)
	if err != nil {
		return c.AbortNotFound("conversation not found")
	}
	if err := h.svc.Erase(c.Request().Context(), conv); err != nil {
		return c.AbortInternalServerError("failed to delete conversation")
	}
	if h.audit != nil {
		h.audit.LogCtx(c, "chat.conversation_deleted", "Chat conversation erased", map[string]any{"conversation_id": conv.UUID})
	}
	return noContent(c)
}

// Ban marks the conversation spam, drops the visitor's further messages and blocks their IP for a day.
func (h *ChatConversationHandler) Ban(c *okapi.Context, req *ChatConversationRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	conv, err := h.find(c, req.ID)
	if err != nil {
		return c.AbortNotFound("conversation not found")
	}
	if err := h.svc.Ban(c.Request().Context(), conv); err != nil {
		return c.AbortInternalServerError("failed to ban visitor")
	}
	if h.audit != nil {
		h.audit.LogCtx(c, "chat.visitor_banned", "Chat visitor banned", map[string]any{"conversation_id": conv.UUID})
	}
	return noContent(c)
}

func (h *ChatConversationHandler) Unban(c *okapi.Context, req *ChatConversationRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	conv, err := h.find(c, req.ID)
	if err != nil {
		return c.AbortNotFound("conversation not found")
	}
	if err := h.svc.Unban(c.Request().Context(), conv); err != nil {
		return c.AbortInternalServerError("failed to lift ban")
	}
	if h.audit != nil {
		h.audit.LogCtx(c, "chat.visitor_unbanned", "Chat visitor ban lifted", map[string]any{"conversation_id": conv.UUID})
	}
	return noContent(c)
}

// Upload takes multipart file, caption and client_msg_id parts.
func (h *ChatConversationHandler) Upload(c *okapi.Context) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	conv, err := h.find(c, c.Param("id"))
	if err != nil {
		return c.AbortNotFound("conversation not found")
	}
	up, err := readUpload(c)
	if err != nil {
		return abortChat(c, err)
	}
	agent := chat.Agent{ID: uint(c.GetInt("user_id"))}
	if u, err := h.users.FindByID(agent.ID); err == nil {
		agent.Name = u.Name
	}
	msg, err := h.svc.PostAgentFile(c.Request().Context(), conv, agent, up)
	if err != nil {
		return abortChat(c, err)
	}
	return created(c, msg)
}

func (h *ChatConversationHandler) Attachment(c *okapi.Context, req *ChatAttachmentAgentRequest) error {
	conv, err := h.find(c, req.ID)
	if err != nil {
		return c.AbortNotFound("conversation not found")
	}
	return serveAttachment(c, h.svc, h.repo, conv, req.Msg, req.Idx)
}

func (h *ChatConversationHandler) Analytics(c *okapi.Context, req *ChatAnalyticsRequest) error {
	ws, err := workspaceOf(c)
	if err != nil {
		return c.AbortBadRequest(err.Error())
	}
	to := time.Now()
	from := to.AddDate(0, 0, -30)
	if req.From != "" {
		if from, err = parseDay(req.From); err != nil {
			return c.AbortBadRequest("from must be RFC 3339 or YYYY-MM-DD")
		}
	}
	if req.To != "" {
		if to, err = parseDay(req.To); err != nil {
			return c.AbortBadRequest("to must be RFC 3339 or YYYY-MM-DD")
		}
	}
	if !from.Before(to) || to.Sub(from) > 366*24*time.Hour {
		return c.AbortBadRequest("the period must be positive and at most a year")
	}
	out, err := h.repo.Analytics(ws, uint(max(req.WidgetID, 0)), from, to)
	if err != nil {
		return c.AbortInternalServerError("failed to compute analytics")
	}
	return ok(c, out)
}

func parseDay(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02", s)
}

// Stream events are hints: the dashboard appends a message whose seq follows its last one, otherwise it refetches.
func (h *ChatConversationHandler) Stream(c *okapi.Context) error {
	bus := h.svc.Bus()
	if bus == nil {
		return c.AbortNotFound("chat stream not configured")
	}
	ws, err := workspaceOf(c)
	if err != nil {
		return c.AbortBadRequest(err.Error())
	}
	ctx := c.Request().Context()
	ch, unsub := bus.Subscribe(ctx, realtime.WorkspaceTopic(ws))
	defer unsub()
	c.ResponseWriter().Header().Set("X-Accel-Buffering", "no")

	out := make(chan okapi.Message, 8)
	out <- okapi.Message{Event: sseEventSystemInfo, Data: okapi.M{"workspace_id": ws, "timestamp": time.Now().UTC().Format(time.RFC3339)}}
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case ev, open := <-ch:
				if !open {
					return
				}
				// Topics are per workspace already; this is defence in depth.
				if !strings.HasPrefix(ev.Type, "chat.") || ev.WorkspaceID != ws {
					continue
				}
				select {
				case out <- okapi.Message{Event: ev.Type, Data: ev.Data}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return c.SSEStreamWithOptions(ctx, out, &okapi.StreamOptions{
		Serializer:   &okapi.JSONSerializer{},
		PingInterval: 25 * time.Second,
	})
}

func (h *ChatConversationHandler) find(c *okapi.Context, id string) (*models.ChatConversation, error) {
	ws, err := workspaceOf(c)
	if err != nil {
		return nil, err
	}
	return h.repo.FindConversation(ws, id)
}

func workspaceOf(c *okapi.Context) (uint, error) {
	scope := getScope(c)
	if scope.WorkspaceID == nil {
		return 0, errors.New("a workspace is required")
	}
	return *scope.WorkspaceID, nil
}

// pageMessages reads the page after a seq, before one, or with neither the newest page.
func pageMessages(repo *repositories.ChatRepository, conv *models.ChatConversation, after, before int64, limit int) ([]models.ChatMessage, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var items []models.ChatMessage
	var err error
	switch {
	case after > 0 || (after == 0 && before == 0 && conv.LastSeq <= int64(limit)):
		items, err = repo.MessagesAfter(conv.ID, after, limit)
	case before > 0:
		items, err = repo.MessagesBefore(conv.ID, before, limit)
	default:
		items, err = repo.MessagesBefore(conv.ID, conv.LastSeq+1, limit)
	}
	if items == nil {
		items = []models.ChatMessage{}
	}
	return items, err
}

func abortChat(c *okapi.Context, err error) error {
	switch {
	case errors.Is(err, chat.ErrEmptyBody), errors.Is(err, chat.ErrBodyTooLong),
		errors.Is(err, chat.ErrNameRequired), errors.Is(err, chat.ErrEmailRequired),
		errors.Is(err, chat.ErrInvalidEmail), errors.Is(err, chat.ErrAttachmentsOff),
		errors.Is(err, chat.ErrAttachmentType), errors.Is(err, chat.ErrInvalidAttributes):
		return c.AbortBadRequest(err.Error())
	case errors.Is(err, chat.ErrAttachmentTooLarge):
		return c.AbortRequestEntityTooLarge(err.Error())
	case errors.Is(err, chat.ErrRateLimited):
		return c.AbortTooManyRequests(err.Error())
	case errors.Is(err, repositories.ErrChatConversationGone):
		return c.AbortNotFound("conversation not found")
	}
	return c.AbortInternalServerError("failed to send message")
}
