// SPDX-FileCopyrightText: 2026 Jonas Kaninda
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

	CreatedAt time.Time      `json:"created_at" gorm:"index"`
	UpdatedAt *time.Time     `json:"updated_at,omitempty"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	Fields      []MessageField          `json:"fields,omitempty" gorm:"-"`
	Attachments []InboundAttachmentMeta `json:"attachments,omitempty" gorm:"-"`

	Form       *Form          `json:"form,omitempty" gorm:"foreignKey:FormID;constraint:false"`
	AssignedTo *ActorRef      `json:"assigned_to,omitempty" gorm:"foreignKey:AssignedToID;references:ID;constraint:false"`
	Replies    []MessageReply `json:"replies,omitempty" gorm:"foreignKey:MessageID"`
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

	AuthorID uint   `json:"author_id" gorm:"index"`
	FromAddr string `json:"from_addr"`
	ToAddr   string `json:"to_addr"`
	Subject  string `json:"subject"`
	HTMLBody string `json:"html_body" gorm:"type:text"`
	TextBody string `json:"text_body" gorm:"type:text"`

	EmailUUID      string `json:"email_uuid,omitempty" gorm:"index"`
	InboundEmailID *uint  `json:"inbound_email_id,omitempty" gorm:"index"`

	CreatedAt time.Time `json:"created_at"`

	Author *ActorRef `json:"author,omitempty" gorm:"foreignKey:AuthorID;references:ID;constraint:false"`
}
