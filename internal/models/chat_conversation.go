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

package models

import (
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

type ChatState string

const (
	ChatStateOpen    ChatState = "open"
	ChatStatePending ChatState = "pending"
	ChatStateClosed  ChatState = "closed"
	ChatStateSpam    ChatState = "spam"
)

// ValidChatState reports whether s is one of the four conversation states.
func ValidChatState(s ChatState) bool {
	switch s {
	case ChatStateOpen, ChatStatePending, ChatStateClosed, ChatStateSpam:
		return true
	}
	return false
}

// ChatConversation is one thread between a visitor and the workspace.
// LastSeq is the sequence number of its newest message; it is bumped in the
// transaction that inserts the message, so sequence numbers have no gaps.
type ChatConversation struct {
	ID          uint   `json:"id" gorm:"primaryKey"`
	UUID        string `json:"uuid" gorm:"type:uuid;default:gen_random_uuid();uniqueIndex;not null"`
	WorkspaceID *uint  `json:"workspace_id,omitempty" gorm:"index;not null"`
	WidgetID    uint   `json:"widget_id" gorm:"index;not null"`
	VisitorID   uint   `json:"visitor_id" gorm:"index;not null"`

	State ChatState `json:"state" gorm:"type:varchar(16);default:'open';index;not null"`
	// Status is the scan verdict on the first message: received, flagged, or
	// quarantined. Quarantined conversations stay out of the default list and
	// notify no one.
	Status      MessageStatus  `json:"status" gorm:"type:varchar(20);default:'received';not null"`
	SpamScore   float64        `json:"spam_score" gorm:"default:0;not null"`
	ScanReasons pq.StringArray `json:"scan_reasons,omitempty" gorm:"type:text[]"`
	// SpamHits counts later messages the scan rejected; three make the
	// conversation spam.
	SpamHits int `json:"spam_hits" gorm:"default:0;not null"`

	AssignedToID *uint `json:"assigned_to_id,omitempty" gorm:"index"`
	// Read receipts: the newest sequence number each side has seen.
	VisitorReadSeq int64 `json:"visitor_read_seq" gorm:"default:0;not null"`
	AgentReadSeq   int64 `json:"agent_read_seq" gorm:"default:0;not null"`
	// EmailedThroughSeq is the newest agent message already emailed to the
	// visitor, so a follow-up email never repeats one.
	EmailedThroughSeq int64  `json:"emailed_through_seq" gorm:"default:0;not null"`
	LastSeq           int64  `json:"last_seq" gorm:"default:0;not null"`
	LastAuthorKind    string `json:"last_author_kind" gorm:"type:varchar(8);default:'';not null"`
	// LastMessagePreview is the start of the newest message, for the list.
	LastMessagePreview string     `json:"last_message_preview" gorm:"type:varchar(160);default:'';not null"`
	LastMessageAt      time.Time  `json:"last_message_at" gorm:"index"`
	LastVisitorMsgAt   *time.Time `json:"last_visitor_msg_at,omitempty"`
	FirstResponseAt    *time.Time `json:"first_response_at,omitempty"`
	ClosedAt           *time.Time `json:"closed_at,omitempty"`
	StartPageURL       string     `json:"start_page_url"`
	// ThreadToken is the plus-address that brings a visitor's emailed answer
	// back into the conversation. NULL for conversations from before it.
	ThreadToken *string `json:"-" gorm:"uniqueIndex"`

	// VisitorLive reports whether the visitor's widget is connected now, and
	// ReplyByEmail whether an agent's reply would reach them by email
	// instead. Computed when read, never stored.
	VisitorLive  bool `json:"visitor_live" gorm:"-"`
	ReplyByEmail bool `json:"reply_by_email" gorm:"-"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt *time.Time     `json:"updated_at,omitempty"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	Visitor    *ChatVisitor   `json:"visitor,omitempty" gorm:"foreignKey:VisitorID;constraint:-"`
	Widget     *ChatWidgetRef `json:"widget,omitempty" gorm:"->;foreignKey:WidgetID;references:ID;constraint:-"`
	AssignedTo *ActorRef      `json:"assigned_to,omitempty" gorm:"->;foreignKey:AssignedToID;references:ID;constraint:-"`
}

// AwaitingReply reports whether the visitor wrote last in an open conversation.
func (c *ChatConversation) AwaitingReply() bool {
	return c.State == ChatStateOpen && c.LastAuthorKind == ChatAuthorVisitor
}
