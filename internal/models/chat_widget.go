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

type ChatWidgetStatus string

const (
	ChatWidgetStatusActive   ChatWidgetStatus = "active"
	ChatWidgetStatusPaused   ChatWidgetStatus = "paused"
	ChatWidgetStatusArchived ChatWidgetStatus = "archived"
)

// ChatWidgetKeyPrefix marks a chat widget's public key, so a key pasted in the
// wrong place is recognisable.
const ChatWidgetKeyPrefix = "pcw_"

// NewChatWidgetKey returns a random public key for the embed snippet.
func NewChatWidgetKey() (string, error) {
	key, err := NewFormPublicKey()
	if err != nil {
		return "", err
	}
	return ChatWidgetKeyPrefix + key, nil
}

const (
	DefaultChatMaxMessageBytes = 4096
	// ScanSourceChatWidget is the ScanSource kind of a chat widget.
	ScanSourceChatWidget = "chat_widget"
)

// ChatWidget is an embeddable live chat: the public key a website loads it
// with, the origins allowed to use it, its appearance, and its spam posture.
type ChatWidget struct {
	ID          uint             `json:"id" gorm:"primaryKey"`
	UUID        string           `json:"uuid" gorm:"type:uuid;default:gen_random_uuid();uniqueIndex;not null"`
	WorkspaceID *uint            `json:"workspace_id,omitempty" gorm:"index;not null"`
	Name        string           `json:"name" gorm:"not null"`
	DisplayName string           `json:"display_name" gorm:"not null;default:''"`
	PublicKey   string           `json:"public_key" gorm:"uniqueIndex;not null"`
	Status      ChatWidgetStatus `json:"status" gorm:"type:varchar(20);default:'active';index;not null"`

	// Chat is JS-only, so a request without an Origin is a script, not a
	// browser: StrictOrigin is on by default.
	AllowedOrigins pq.StringArray `json:"allowed_origins" gorm:"type:text[]"`
	StrictOrigin   bool           `json:"strict_origin" gorm:"default:true;not null"`

	// Appearance, served by the public config endpoint. AccentColor and
	// AvatarURL end up in the widget's shadow DOM on the customer's page, so
	// they are validated on save.
	Title          string `json:"title"`
	Greeting       string `json:"greeting"`
	AccentColor    string `json:"accent_color" gorm:"size:7"`
	Position       string `json:"position" gorm:"type:varchar(8);default:'right';not null"`
	AvatarURL      string `json:"avatar_url"`
	ShowAgentNames bool   `json:"show_agent_names" gorm:"default:true;not null"`

	// Pre-chat form. Email is always required while the widget is offline.
	RequireEmail bool `json:"require_email" gorm:"default:false;not null"`
	RequireName  bool `json:"require_name" gorm:"default:false;not null"`

	// Availability. The widget is online when an agent is and the time is
	// inside business hours (empty hours: always). Offline, it either takes
	// messages to answer by email or hides.
	BusinessHours   BusinessHours `json:"business_hours" gorm:"type:jsonb"`
	OfflineMode     string        `json:"offline_mode" gorm:"type:varchar(8);default:'email';not null"`
	OfflineGreeting string        `json:"offline_greeting"`

	// ReplyFrom sends agent replies to visitors who left; it must be on an
	// ownership-verified domain of the workspace.
	ReplyFrom     string `json:"reply_from"`
	ReplyFromName string `json:"reply_from_name"`

	// Attachments: images and PDFs up to 5 MB, stored in the blob store.
	AllowAttachments bool `json:"allow_attachments" gorm:"default:false;not null"`

	// Auto-assign gives each new conversation to an online agent in turn,
	// skipping agents who already hold MaxOpenPerAgent open conversations
	// (0: no cap).
	AutoAssign      bool `json:"auto_assign" gorm:"default:false;not null"`
	MaxOpenPerAgent int  `json:"max_open_per_agent" gorm:"default:0;not null"`

	// IdentitySecretEnc signs PostaChat.identify() on the customer's server.
	// Stored encrypted, shown once when rotated; HasIdentitySecret says
	// whether one is set.
	IdentitySecretEnc string `json:"-"`
	HasIdentitySecret bool   `json:"has_identity_secret" gorm:"-"`

	ScanEnabled         bool    `json:"scan_enabled" gorm:"default:true;not null"`
	FlagThreshold       float64 `json:"flag_threshold" gorm:"default:3;not null"`
	QuarantineThreshold float64 `json:"quarantine_threshold" gorm:"default:6;not null"`
	RejectThreshold     float64 `json:"reject_threshold" gorm:"default:10;not null"`
	MaxMessageBytes     int     `json:"max_message_bytes" gorm:"default:4096;not null"`
	// RetentionDays keeps closed and spam conversations this long; 0 means the
	// platform's POSTA_CHAT_RETENTION_DAYS, which it can only lower.
	RetentionDays int `json:"retention_days" gorm:"default:0;not null"`

	LastEditedByID *uint          `json:"last_edited_by_id,omitempty" gorm:"index"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      *time.Time     `json:"updated_at,omitempty"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`

	LastEditedBy *ActorRef `json:"last_edited_by,omitempty" gorm:"->;foreignKey:LastEditedByID;references:ID;constraint:-"`
}

// Offline modes.
const (
	ChatOfflineEmail = "email"
	ChatOfflineHide  = "hide"
)

func (w *ChatWidget) IsActive() bool { return w.Status == ChatWidgetStatusActive }

func (w *ChatWidget) AfterFind(*gorm.DB) error {
	w.HasIdentitySecret = w.IdentitySecretEnc != ""
	return nil
}

// HidesWhenOffline reports whether the widget disappears while no one can answer.
func (w *ChatWidget) HidesWhenOffline() bool { return w.OfflineMode == ChatOfflineHide }

// MessageLimit is the largest message body the widget accepts, in bytes.
func (w *ChatWidget) MessageLimit() int {
	if w.MaxMessageBytes <= 0 {
		return DefaultChatMaxMessageBytes
	}
	return w.MaxMessageBytes
}

// OriginPolicy returns the origin allowlist and whether a missing Origin is
// refused, for messages.OriginAllowed.
func (w *ChatWidget) OriginPolicy() ([]string, bool) { return w.AllowedOrigins, w.StrictOrigin }

// ScanningEnabled, ScanThresholds, and ScanSource make a ChatWidget a
// messagescan.Policy.
func (w *ChatWidget) ScanningEnabled() bool { return w == nil || w.ScanEnabled }

func (w *ChatWidget) ScanThresholds() (flag, quarantine, reject float64) {
	if w == nil {
		return 0, 0, 0
	}
	return w.FlagThreshold, w.QuarantineThreshold, w.RejectThreshold
}

func (w *ChatWidget) ScanSource() ScanSource {
	if w == nil {
		return ScanSource{}
	}
	return ScanSource{Kind: ScanSourceChatWidget, ID: w.ID}
}

// ChatWidgetRef is the name of the widget a conversation came through.
type ChatWidgetRef struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

func (ChatWidgetRef) TableName() string { return "chat_widgets" }
