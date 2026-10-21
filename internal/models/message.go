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

type MessageStatus string

const (
	MessageStatusReceived    MessageStatus = "received"
	MessageStatusFlagged     MessageStatus = "flagged"
	MessageStatusQuarantined MessageStatus = "quarantined"
	MessageStatusRejected    MessageStatus = "rejected"
)

type MessageState string

const (
	MessageStateNew     MessageState = "new"
	MessageStateOpen    MessageState = "open"
	MessageStateReplied MessageState = "replied"
	MessageStateClosed  MessageState = "closed"
	MessageStateSpam    MessageState = "spam"
)

type MessageReplyKind string

const (
	MessageReplyKindOperator MessageReplyKind = "operator"
	MessageReplyKindInbound  MessageReplyKind = "inbound"
)

type MessageField struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Message struct {
	ID          uint   `json:"id" gorm:"primaryKey"`
	UUID        string `json:"uuid" gorm:"type:uuid;default:gen_random_uuid();uniqueIndex;not null"`
	WorkspaceID *uint  `json:"workspace_id,omitempty" gorm:"index;not null"`
	FormID      uint   `json:"form_id" gorm:"index;not null"`

	SenderEmail string `json:"sender_email" gorm:"index"`
	SenderName  string `json:"sender_name"`
	SenderPhone string `json:"sender_phone"`
	Subject     string `json:"subject"`
	Body        string `json:"body" gorm:"type:text"`

	FieldsJSON      string `json:"-" gorm:"type:text"`
	AttachmentsJSON string `json:"-" gorm:"type:text"`

	ClientIP  string `json:"client_ip,omitempty" gorm:"size:45"`
	UserAgent string `json:"user_agent,omitempty"`
	Referer   string `json:"referer,omitempty"`
	Origin    string `json:"origin,omitempty"`

	Status      MessageStatus  `json:"status" gorm:"type:varchar(20);default:'received';index;not null"`
	State       MessageState   `json:"state" gorm:"type:varchar(20);default:'new';index;not null"`
	SpamScore   float64        `json:"spam_score" gorm:"default:0;not null"`
	ScanReasons pq.StringArray `json:"scan_reasons" gorm:"type:text[]"`

	ThreadToken   string `json:"-" gorm:"uniqueIndex"`
	RootMessageID string `json:"-"`
	DedupHash     string `json:"-" gorm:"index"`

	NotifiedAt   *time.Time `json:"notified_at,omitempty"`
	AssignedToID *uint      `json:"assigned_to_id,omitempty" gorm:"index"`
	ReadAt       *time.Time `json:"read_at,omitempty"`
	RepliedAt    *time.Time `json:"replied_at,omitempty"`
	ReplyCount   int        `json:"reply_count" gorm:"default:0;not null"`

	// WebhookDispatchedAt records that message:process handed the submission to
	// the webhook dispatcher, so a retried task never fires the webhook twice.
	// Delivery itself is tracked in webhook_deliveries.
	WebhookDispatchedAt *time.Time `json:"-"`

	CreatedAt time.Time      `json:"created_at" gorm:"index"`
	UpdatedAt *time.Time     `json:"updated_at,omitempty"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	Fields      []MessageField          `json:"fields,omitempty" gorm:"-"`
	Attachments []InboundAttachmentMeta `json:"attachments,omitempty" gorm:"-"`

	// GORM creates these foreign keys whatever the tag (only "constraint:-"
	// skips one), so declare the cascade the schema relies on: deleting a form
	// or a message must take its messages or replies with it.
	Form       *Form          `json:"form,omitempty" gorm:"foreignKey:FormID;constraint:OnDelete:CASCADE"`
	AssignedTo *ActorRef      `json:"assigned_to,omitempty" gorm:"->;foreignKey:AssignedToID;references:ID;constraint:false"`
	Replies    []MessageReply `json:"replies,omitempty" gorm:"foreignKey:MessageID;constraint:OnDelete:CASCADE"`
}

func (m *Message) IsSpam() bool {
	return m.Status == MessageStatusQuarantined || m.Status == MessageStatusRejected
}

func (m *Message) CanReply() bool {
	return m.SenderEmail != "" && m.Status != MessageStatusRejected
}

type MessageReply struct {
	ID          uint             `json:"id" gorm:"primaryKey"`
	UUID        string           `json:"uuid" gorm:"type:uuid;default:gen_random_uuid();uniqueIndex;not null"`
	MessageID   uint             `json:"message_id" gorm:"index;not null"`
	WorkspaceID *uint            `json:"workspace_id,omitempty" gorm:"index;not null"`
	Kind        MessageReplyKind `json:"kind" gorm:"type:varchar(16);not null"`

	// AuthorID is the operator who wrote an outbound reply; nil for an inbound
	// reply. It must stay nullable: author_id carries a foreign key to users.
	AuthorID *uint  `json:"author_id,omitempty" gorm:"index"`
	FromAddr string `json:"from_addr"`
	ToAddr   string `json:"to_addr"`
	Subject  string `json:"subject"`
	HTMLBody string `json:"html_body" gorm:"type:text"`
	TextBody string `json:"text_body" gorm:"type:text"`

	EmailUUID      string `json:"email_uuid,omitempty" gorm:"index"`
	InboundEmailID *uint  `json:"inbound_email_id,omitempty" gorm:"index"`

	// MessageIDHeader is the RFC 5322 Message-ID this reply was sent or received
	// with. Later replies chain to it through In-Reply-To and References.
	MessageIDHeader string `json:"-"`
	// SenderMismatch marks an inbound reply whose sender is not the original
	// submitter: a forward, or a spoof using a leaked thread address.
	SenderMismatch bool `json:"sender_mismatch,omitempty" gorm:"default:false;not null"`

	CreatedAt time.Time `json:"created_at"`

	Author *ActorRef `json:"author,omitempty" gorm:"->;foreignKey:AuthorID;references:ID;constraint:false"`
}
