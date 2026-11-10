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

// ChatWidgetKeyPrefix makes a key pasted in the wrong place recognisable.
const ChatWidgetKeyPrefix = "pcw_"

func NewChatWidgetKey() (string, error) {
	key, err := NewFormPublicKey()
	if err != nil {
		return "", err
	}
	return ChatWidgetKeyPrefix + key, nil
}

const (
	DefaultChatMaxMessageBytes = 4096
	ScanSourceChatWidget       = "chat_widget"
)

type ChatWidget struct {
	ID          uint             `json:"id" gorm:"primaryKey"`
	UUID        string           `json:"uuid" gorm:"type:uuid;default:gen_random_uuid();uniqueIndex;not null"`
	WorkspaceID *uint            `json:"workspace_id,omitempty" gorm:"index;not null"`
	Name        string           `json:"name" gorm:"not null"`
	DisplayName string           `json:"display_name" gorm:"not null;default:''"`
	PublicKey   string           `json:"public_key" gorm:"uniqueIndex;not null"`
	Status      ChatWidgetStatus `json:"status" gorm:"type:varchar(20);default:'active';index;not null"`

	// Chat is JS-only, so a request without an Origin is a script, not a browser: StrictOrigin is on by default.
	AllowedOrigins pq.StringArray `json:"allowed_origins" gorm:"type:text[]"`
	StrictOrigin   bool           `json:"strict_origin" gorm:"default:true;not null"`

	// AccentColor and AvatarURL end up in the widget's shadow DOM on the customer's page, so they are validated on save.
	Title          string `json:"title"`
	Greeting       string `json:"greeting"`
	AccentColor    string `json:"accent_color" gorm:"size:7"`
	Position       string `json:"position" gorm:"type:varchar(8);default:'right';not null"`
	AvatarURL      string `json:"avatar_url"`
	ShowAgentNames bool   `json:"show_agent_names" gorm:"default:true;not null"`
	// Home is NULL for widgets created before it existed, which keeps them a single thread.
	Home     ChatHome     `json:"home" gorm:"type:jsonb"`
	Theme    string       `json:"theme" gorm:"type:varchar(8);default:'auto';not null"`
	Launcher ChatLauncher `json:"launcher" gorm:"type:jsonb"`
	Teaser   ChatTeaser   `json:"teaser" gorm:"type:jsonb"`
	// OneOpenConversation keeps a visitor to one thread, as before the home screen; off, Messages offers a new one.
	OneOpenConversation bool `json:"one_open_conversation" gorm:"default:true;not null"`

	// Email is always required while the widget is offline.
	RequireEmail bool `json:"require_email" gorm:"default:false;not null"`
	RequireName  bool `json:"require_name" gorm:"default:false;not null"`

	// Online needs an online agent and the time inside business hours (empty: always); offline it takes messages or hides.
	BusinessHours   BusinessHours `json:"business_hours" gorm:"type:jsonb"`
	OfflineMode     string        `json:"offline_mode" gorm:"type:varchar(8);default:'email';not null"`
	OfflineGreeting string        `json:"offline_greeting"`

	// ReplyFrom must be on an ownership-verified domain of the workspace.
	ReplyFrom     string `json:"reply_from"`
	ReplyFromName string `json:"reply_from_name"`

	// Attachments: images and PDFs up to 5 MB, stored in the blob store.
	AllowAttachments bool `json:"allow_attachments" gorm:"default:false;not null"`

	// Auto-assign rotates new conversations over online agents holding fewer than MaxOpenPerAgent (0: no cap).
	AutoAssign      bool `json:"auto_assign" gorm:"default:false;not null"`
	MaxOpenPerAgent int  `json:"max_open_per_agent" gorm:"default:0;not null"`

	// IdentitySecretEnc is stored encrypted and shown once when rotated.
	IdentitySecretEnc string `json:"-"`
	HasIdentitySecret bool   `json:"has_identity_secret" gorm:"-"`

	ScanEnabled         bool    `json:"scan_enabled" gorm:"default:true;not null"`
	FlagThreshold       float64 `json:"flag_threshold" gorm:"default:3;not null"`
	QuarantineThreshold float64 `json:"quarantine_threshold" gorm:"default:6;not null"`
	RejectThreshold     float64 `json:"reject_threshold" gorm:"default:10;not null"`
	MaxMessageBytes     int     `json:"max_message_bytes" gorm:"default:4096;not null"`
	// RetentionDays can only lower POSTA_CHAT_RETENTION_DAYS; 0 uses the platform value.
	RetentionDays int `json:"retention_days" gorm:"default:0;not null"`

	LastEditedByID *uint          `json:"last_edited_by_id,omitempty" gorm:"index"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      *time.Time     `json:"updated_at,omitempty"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`

	LastEditedBy *ActorRef `json:"last_edited_by,omitempty" gorm:"->;foreignKey:LastEditedByID;references:ID;constraint:-"`
}

const (
	ChatOfflineEmail = "email"
	ChatOfflineHide  = "hide"
)

const (
	ChatThemeAuto  = "auto"
	ChatThemeLight = "light"
	ChatThemeDark  = "dark"
)

func (w *ChatWidget) IsActive() bool { return w.Status == ChatWidgetStatusActive }

func (w *ChatWidget) AfterFind(*gorm.DB) error {
	w.HasIdentitySecret = w.IdentitySecretEnc != ""
	return nil
}

func (w *ChatWidget) HidesWhenOffline() bool { return w.OfflineMode == ChatOfflineHide }

// MessageLimit is the largest message body the widget accepts, in bytes.
func (w *ChatWidget) MessageLimit() int {
	if w.MaxMessageBytes <= 0 {
		return DefaultChatMaxMessageBytes
	}
	return w.MaxMessageBytes
}

// OriginPolicy returns the allowlist and whether a missing Origin is refused.
func (w *ChatWidget) OriginPolicy() ([]string, bool) { return w.AllowedOrigins, w.StrictOrigin }

// ScanningEnabled, ScanThresholds and ScanSource make a ChatWidget a messagescan.Policy.
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
