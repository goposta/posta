// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import (
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

type FormStatus string

const (
	FormStatusActive   FormStatus = "active"
	FormStatusPaused   FormStatus = "paused"
	FormStatusArchived FormStatus = "archived"
)

type NotifyMode string

const (
	NotifyModeImmediate NotifyMode = "immediate"
	NotifyModeHourly    NotifyMode = "hourly"
	NotifyModeDaily     NotifyMode = "daily"
	NotifyModeOff       NotifyMode = "off"
)

const (
	DefaultHoneypotField = "_gotcha"
	DefaultMaxBodyBytes  = 65536
	DefaultMaxFields     = 40
)

type Form struct {
	ID          uint       `json:"id" gorm:"primaryKey"`
	UUID        string     `json:"uuid" gorm:"type:uuid;default:gen_random_uuid();uniqueIndex;not null"`
	WorkspaceID *uint      `json:"workspace_id,omitempty" gorm:"index;not null"`
	Name        string     `json:"name" gorm:"not null"`
	Slug        string     `json:"slug" gorm:"not null"`
	Description string     `json:"description"`
	PublicKey   string     `json:"public_key" gorm:"uniqueIndex;not null"`
	Status      FormStatus `json:"status" gorm:"type:varchar(20);default:'active';index;not null"`

	AllowedOrigins   pq.StringArray `json:"allowed_origins" gorm:"type:text[]"`
	StrictOrigin     bool           `json:"strict_origin" gorm:"default:false;not null"`
	RedirectURL      string         `json:"redirect_url"`
	MaxBodyBytes     int64          `json:"max_body_bytes" gorm:"default:65536;not null"`
	MaxFields        int            `json:"max_fields" gorm:"default:40;not null"`
	AllowAttachments bool           `json:"allow_attachments" gorm:"default:false;not null"`

	HoneypotField  string `json:"honeypot_field" gorm:"default:'_gotcha';not null"`
	RequireNonce   bool   `json:"require_nonce" gorm:"default:false;not null"`
	MinFillSeconds int    `json:"min_fill_seconds" gorm:"default:3;not null"`

	ScanEnabled         bool    `json:"scan_enabled" gorm:"default:true;not null"`
	FlagThreshold       float64 `json:"flag_threshold" gorm:"default:3;not null"`
	QuarantineThreshold float64 `json:"quarantine_threshold" gorm:"default:6;not null"`
	RejectThreshold     float64 `json:"reject_threshold" gorm:"default:10;not null"`

	NotifyEnabled   bool           `json:"notify_enabled" gorm:"default:true;not null"`
	NotifyEmails    pq.StringArray `json:"notify_emails" gorm:"type:text[]"`
	NotifyMode      NotifyMode     `json:"notify_mode" gorm:"type:varchar(16);default:'immediate';not null"`
	NotifyOnFlagged bool           `json:"notify_on_flagged" gorm:"default:true;not null"`

	ReplyFrom     string `json:"reply_from"`
	ReplyFromName string `json:"reply_from_name"`

	RetentionDays  int   `json:"retention_days" gorm:"default:0;not null"`
	LastEditedByID *uint `json:"last_edited_by_id,omitempty" gorm:"index"`

	MessageCount  int64          `json:"message_count" gorm:"default:0;not null"`
	SpamCount     int64          `json:"spam_count" gorm:"default:0;not null"`
	LastMessageAt *time.Time     `json:"last_message_at,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     *time.Time     `json:"updated_at,omitempty"`
	DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`

	LastEditedBy *ActorRef `json:"last_edited_by,omitempty" gorm:"foreignKey:LastEditedByID;references:ID;constraint:false"`
}

func (f *Form) IsActive() bool { return f.Status == FormStatusActive }

func (f *Form) Honeypot() string {
	if f.HoneypotField == "" {
		return DefaultHoneypotField
	}
	return f.HoneypotField
}

func (f *Form) BodyLimit() int64 {
	if f.MaxBodyBytes <= 0 {
		return DefaultMaxBodyBytes
	}
	return f.MaxBodyBytes
}

func (f *Form) FieldLimit() int {
	if f.MaxFields <= 0 {
		return DefaultMaxFields
	}
	return f.MaxFields
}
