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

package chat

import (
	"context"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/realtime"
	"github.com/jkaninda/logger"
)

// Live event types: agents receive them on the workspace topic, visitors on their conversation's topic.
const (
	EventMessage      = "chat.message"
	EventConversation = "chat.conversation"
	// Ephemeral: never stored, no sequence number.
	EventTyping   = "chat.typing"
	EventRead     = "chat.read"
	EventPresence = "chat.presence"
	EventVisitor  = "chat.visitor"
)

// Webhook events, sent from the worker so a slow subscriber never delays a chat.
const (
	WebhookStarted  = "chat.conversation.started"
	WebhookMessage  = "chat.message.created"
	WebhookAssigned = "chat.conversation.assigned"
	WebhookClosed   = "chat.conversation.closed"
)

// webhook queues a webhook event; messageID is 0 for conversation events.
func (s *Service) webhook(event string, conv *models.ChatConversation, messageID uint) {
	if s.enqueuer == nil || conv.Status == models.MessageStatusQuarantined {
		return
	}
	if err := s.enqueuer.EnqueueChatWebhook(event, conv.ID, messageID); err != nil {
		logger.Error("chat: failed to enqueue webhook", "event", event, "conversation_id", conv.ID, "error", err)
	}
}

// TypingEvent says one side is writing; Name is the agent's, and only when shown.
type TypingEvent struct {
	ConversationID string `json:"conversation_id"`
	Author         string `json:"author"`
	Name           string `json:"name,omitempty"`
}

type ReadEvent struct {
	ConversationID string `json:"conversation_id"`
	Reader         string `json:"reader"`
	Seq            int64  `json:"seq"`
}

type PresenceEvent struct {
	UserID uint   `json:"user_id"`
	Status string `json:"status"`
}

type VisitorEvent struct {
	ConversationID string `json:"conversation_id"`
	Live           bool   `json:"live"`
}

// VisitorMessage omits agent IDs and emails, and the agent's name unless the widget shows names.
type VisitorMessage struct {
	ID          string    `json:"id"`
	Seq         int64     `json:"seq"`
	Author      string    `json:"author" enum:"visitor,agent,system"`
	AgentName   string    `json:"agent_name,omitempty"`
	Body        string    `json:"body"`
	ClientMsgID string    `json:"client_msg_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	// Attachments are fetched by message ID and index.
	Attachments models.ChatAttachments `json:"attachments,omitempty"`
}

func ToVisitorMessage(m *models.ChatMessage, showAgentNames bool) VisitorMessage {
	out := VisitorMessage{
		ID:          m.UUID,
		Seq:         m.Seq,
		Author:      m.AuthorKind,
		Body:        m.Body,
		ClientMsgID: m.ClientMsgID,
		CreatedAt:   m.CreatedAt,
		Attachments: m.Attachments,
	}
	if showAgentNames && m.AuthorKind == models.ChatAuthorAgent && m.Author != nil {
		out.AgentName = m.Author.Name
	}
	return out
}

func ToVisitorMessages(items []models.ChatMessage, showAgentNames bool) []VisitorMessage {
	out := make([]VisitorMessage, 0, len(items))
	for i := range items {
		out = append(out, ToVisitorMessage(&items[i], showAgentNames))
	}
	return out
}

type VisitorConversation struct {
	ID            string    `json:"id"`
	State         string    `json:"state"`
	LastSeq       int64     `json:"last_seq"`
	AgentReadSeq  int64     `json:"agent_read_seq" doc:"Newest message an agent has seen"`
	LastMessageAt time.Time `json:"last_message_at"`
	CreatedAt     time.Time `json:"created_at"`
}

func ToVisitorConversation(c *models.ChatConversation) VisitorConversation {
	state := c.State
	// A visitor is never told their conversation was marked as spam.
	if state == models.ChatStateSpam {
		state = models.ChatStateClosed
	}
	return VisitorConversation{
		ID: c.UUID, State: string(state), LastSeq: c.LastSeq, AgentReadSeq: c.AgentReadSeq,
		LastMessageAt: c.LastMessageAt, CreatedAt: c.CreatedAt,
	}
}

type AgentMessageEvent struct {
	ConversationID string              `json:"conversation_id"`
	Message        *models.ChatMessage `json:"message"`
}

type ConversationEvent struct {
	ConversationID string           `json:"conversation_id"`
	Started        bool             `json:"started,omitempty"`
	State          models.ChatState `json:"state"`
	AssignedToID   *uint            `json:"assigned_to_id"`
	LastSeq        int64            `json:"last_seq"`
	VisitorName    string           `json:"visitor_name,omitempty"`
	Preview        string           `json:"preview,omitempty"`
}

// A failed publish costs latency, never the message: both sides catch up from the database by sequence number.
func (s *Service) publishMessage(ctx context.Context, conv *models.ChatConversation, widget *models.ChatWidget, msg *models.ChatMessage) {
	if s.bus == nil || conv.WorkspaceID == nil {
		return
	}
	ws := *conv.WorkspaceID
	s.publish(ctx, realtime.Event{
		Type: EventMessage, WorkspaceID: ws, Topic: realtime.WorkspaceTopic(ws), Seq: msg.Seq,
		Data: AgentMessageEvent{ConversationID: conv.UUID, Message: msg},
	})
	showNames := widget == nil || widget.ShowAgentNames
	s.publish(ctx, realtime.Event{
		Type: EventMessage, WorkspaceID: ws, Topic: realtime.ChatConversationTopic(conv.ID), Seq: msg.Seq,
		Data: ToVisitorMessage(msg, showNames),
	})
}

// publishConversation also reaches the visitor unless the change only matters to agents.
func (s *Service) publishConversation(ctx context.Context, conv *models.ChatConversation, started bool, preview string) {
	if s.bus == nil || conv.WorkspaceID == nil {
		return
	}
	ws := *conv.WorkspaceID
	ev := ConversationEvent{
		ConversationID: conv.UUID, Started: started, State: conv.State,
		AssignedToID: conv.AssignedToID, LastSeq: conv.LastSeq, Preview: preview,
	}
	if conv.Visitor != nil {
		ev.VisitorName = conv.Visitor.Name
	}
	s.publish(ctx, realtime.Event{
		Type: EventConversation, WorkspaceID: ws, Topic: realtime.WorkspaceTopic(ws), Data: ev,
	})
	if !started {
		s.publish(ctx, realtime.Event{
			Type: EventConversation, WorkspaceID: ws, Topic: realtime.ChatConversationTopic(conv.ID),
			Data: ToVisitorConversation(conv),
		})
	}
}

func (s *Service) publish(ctx context.Context, ev realtime.Event) {
	if err := s.bus.Publish(ctx, ev); err != nil {
		logger.Warn("chat: failed to publish event", "type", ev.Type, "topic", ev.Topic, "error", err)
	}
}
