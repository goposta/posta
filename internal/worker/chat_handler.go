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

package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/goposta/posta/internal/dto"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/chat"
	"github.com/goposta/posta/internal/services/notification"
	"github.com/goposta/posta/internal/services/webhook"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

type chatStore interface {
	FindConversationByID(id uint) (*models.ChatConversation, error)
	FirstMessage(conversationID uint) (*models.ChatMessage, error)
	FindWidgetByID(id uint) (*models.ChatWidget, error)
	FindMessageByID(id uint) (*models.ChatMessage, error)
}

type chatNotifier interface {
	IsConfigured() bool
	SendToUser(userID uint, subject, templateName string, data map[string]any) error
	SendToWorkspaceAdmins(workspaceID uint, subject, templateName string, data map[string]any) error
}

type agentPresence interface {
	AnyOnline(ctx context.Context, workspaceID uint) bool
}

type offlineSender interface {
	SendOfflineEmail(ctx context.Context, conversationID uint, seq int64) error
}

// unansweredDebounce is the most often one conversation raises the
// unanswered alert.
const unansweredDebounce = time.Hour

// ChatNotifyHandler tells a workspace's owners and admins that a visitor
// started a chat. Agents watching the dashboard already saw it live; this
// reaches the ones who are not.
type ChatNotifyHandler struct {
	repo       chatStore
	notifier   chatNotifier
	appURL     string
	presence   agentPresence
	offline    offlineSender
	dispatcher jsonDispatcher
	// once reports whether key is new, remembering it for ttl.
	once func(ctx context.Context, key string, ttl time.Duration) bool
}

// SetDispatcher enables chat webhooks.
func (h *ChatNotifyHandler) SetDispatcher(d *webhook.Dispatcher) {
	if d != nil {
		h.dispatcher = d
	}
}

// SetChat enables the follow-up email and presence checks, and Redis for
// the unanswered alert's debounce.
func (h *ChatNotifyHandler) SetChat(svc *chat.Service, rdb *redis.Client) {
	h.presence, h.offline = svc.Presence(), svc
	h.once = func(ctx context.Context, key string, ttl time.Duration) bool {
		ok, err := rdb.SetNX(ctx, key, 1, ttl).Result()
		return err != nil || ok
	}
}

func NewChatNotifyHandler(repo *repositories.ChatRepository, notifier *notification.Service, appURL string) *ChatNotifyHandler {
	h := &ChatNotifyHandler{repo: repo, appURL: strings.TrimSuffix(appURL, "/")}
	if notifier != nil {
		h.notifier = notifier
	}
	return h
}

func (h *ChatNotifyHandler) ProcessTask(_ context.Context, t *asynq.Task) error {
	var p ChatNotifyPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("decode chat notify payload: %w: %w", err, asynq.SkipRetry)
	}
	if h.notifier == nil || !h.notifier.IsConfigured() {
		return nil
	}
	conv, err := h.repo.FindConversationByID(p.ConversationID)
	if err != nil {
		// Erased before the worker got to it: nothing to tell.
		return nil
	}
	if conv.WorkspaceID == nil || conv.Status == models.MessageStatusQuarantined || conv.State == models.ChatStateSpam {
		return nil
	}
	// Agents online saw the chat arrive in the dashboard; email is for
	// when nobody is watching.
	if h.presence != nil && h.presence.AnyOnline(context.Background(), *conv.WorkspaceID) {
		return nil
	}
	first, err := h.repo.FirstMessage(conv.ID)
	if err != nil {
		return fmt.Errorf("first message of conversation %d: %w", conv.ID, err)
	}
	data := chatNotifyData(conv, first, h.appURL)
	subject := "New chat"
	if name, _ := data["VisitorName"].(string); name != "" {
		subject = "New chat from " + name
	}
	return h.notifier.SendToWorkspaceAdmins(*conv.WorkspaceID, subject, notification.TemplateNewChat, data)
}

func chatNotifyData(conv *models.ChatConversation, first *models.ChatMessage, appURL string) map[string]any {
	data := map[string]any{
		"Body":      first.Body,
		"PageURL":   conv.StartPageURL,
		"Flagged":   conv.Status == models.MessageStatusFlagged,
		"SpamScore": conv.SpamScore,
	}
	if conv.Widget != nil {
		data["WidgetName"] = conv.Widget.DisplayName
	}
	if conv.Visitor != nil {
		data["VisitorName"] = conv.Visitor.Name
		data["VisitorEmail"] = conv.Visitor.Email
	}
	if appURL != "" {
		data["ConversationURL"] = appURL + "/chat/" + conv.UUID
	}
	return data
}

// ProcessOfflineEmail sends an agent's replies to a visitor who left.
func (h *ChatNotifyHandler) ProcessOfflineEmail(ctx context.Context, t *asynq.Task) error {
	var p ChatMessagePayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("decode chat offline email payload: %w: %w", err, asynq.SkipRetry)
	}
	if h.offline == nil {
		return nil
	}
	return h.offline.SendOfflineEmail(ctx, p.ConversationID, p.Seq)
}

// ProcessUnanswered tells the assignee, or the admins when no one is
// assigned, that a visitor's message is still waiting. It only fires for the
// visitor's newest message, while the conversation is open and the visitor
// wrote last, and at most once an hour per conversation.
func (h *ChatNotifyHandler) ProcessUnanswered(ctx context.Context, t *asynq.Task) error {
	var p ChatMessagePayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("decode chat unanswered payload: %w: %w", err, asynq.SkipRetry)
	}
	if h.notifier == nil || !h.notifier.IsConfigured() {
		return nil
	}
	conv, err := h.repo.FindConversationByID(p.ConversationID)
	if err != nil || !stillUnanswered(conv, p.Seq) {
		return nil
	}
	if h.once != nil && !h.once(ctx, fmt.Sprintf("chat:unanswered:sent:%d", conv.ID), unansweredDebounce) {
		return nil
	}
	data := chatNotifyData(conv, &models.ChatMessage{Body: conv.LastMessagePreview}, h.appURL)
	data["Unanswered"] = true
	subject := "A chat is waiting for a reply"
	if conv.AssignedToID != nil {
		return h.notifier.SendToUser(*conv.AssignedToID, subject, notification.TemplateChatUnanswered, data)
	}
	return h.notifier.SendToWorkspaceAdmins(*conv.WorkspaceID, subject, notification.TemplateChatUnanswered, data)
}

// stillUnanswered reports whether the visitor's message seq is still the
// newest message of an open, non-spam conversation.
func stillUnanswered(conv *models.ChatConversation, seq int64) bool {
	return conv.WorkspaceID != nil &&
		conv.State == models.ChatStateOpen &&
		conv.Status != models.MessageStatusQuarantined &&
		conv.LastAuthorKind == models.ChatAuthorVisitor &&
		conv.LastSeq == seq
}

// ProcessWebhook sends a chat webhook event, built from the database when
// the task runs.
func (h *ChatNotifyHandler) ProcessWebhook(_ context.Context, t *asynq.Task) error {
	var p ChatWebhookPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("decode chat webhook payload: %w: %w", err, asynq.SkipRetry)
	}
	if h.dispatcher == nil {
		return nil
	}
	conv, err := h.repo.FindConversationByID(p.ConversationID)
	if err != nil || conv.WorkspaceID == nil {
		return nil // erased since
	}
	w, err := h.repo.FindWidgetByID(conv.WidgetID)
	if err != nil {
		return nil
	}
	var msg *models.ChatMessage
	if p.MessageID != 0 {
		if msg, err = h.repo.FindMessageByID(p.MessageID); err != nil {
			return nil
		}
	}
	body, err := json.Marshal(chatWebhookBody(p.Event, conv, w, msg, time.Now()))
	if err != nil {
		return err
	}
	h.dispatcher.DispatchJSON(0, conv.WorkspaceID, p.Event, body, "")
	return nil
}

func chatWebhookBody(event string, conv *models.ChatConversation, w *models.ChatWidget, msg *models.ChatMessage, now time.Time) dto.ChatWebhookEvent {
	out := dto.ChatWebhookEvent{
		Event:     event,
		Timestamp: now.UTC().Format(time.RFC3339),
		Conversation: dto.ChatWebhookConv{
			ID: conv.UUID, WidgetID: w.UUID, WidgetName: w.DisplayName,
			State: string(conv.State), Status: string(conv.Status),
			AssignedToID: conv.AssignedToID, StartPageURL: conv.StartPageURL,
			CreatedAt: conv.CreatedAt.UTC().Format(time.RFC3339),
		},
	}
	if v := conv.Visitor; v != nil {
		out.Conversation.Visitor = dto.ChatWebhookVisitor{
			ID: v.UUID, Name: v.Name, Email: v.Email, EmailVerified: v.EmailVerified,
			ExternalID: v.ExternalID, Attributes: v.Attributes,
		}
	}
	if msg != nil {
		m := &dto.ChatWebhookMessage{
			ID: msg.UUID, Seq: msg.Seq, AuthorKind: msg.AuthorKind, AuthorID: msg.AuthorID,
			Body: msg.Body, Channel: msg.Channel, CreatedAt: msg.CreatedAt.UTC().Format(time.RFC3339),
		}
		for _, a := range msg.Attachments {
			m.Attachments = append(m.Attachments, dto.ChatWebhookAttachment{Filename: a.Filename, ContentType: a.ContentType, Size: a.Size})
		}
		out.Message = m
	}
	return out
}
