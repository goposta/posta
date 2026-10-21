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

	LastEditedBy *ActorRef `json:"last_edited_by,omitempty" gorm:"->;foreignKey:LastEditedByID;references:ID;constraint:false"`
}

func (f *Form) IsActive() bool { return f.Status == FormStatusActive }

// ScanSource identifies what a submission arrived through, so the scanner can
// apply that source's filters and count repeat submissions per source.
type ScanSource struct {
	Kind string `json:"kind"`
	ID   uint   `json:"id"`
}

// ScanSourceForm is the ScanSource kind of a web form.
const ScanSourceForm = "form"

// ScanningEnabled, ScanThresholds, and ScanSource make a Form a
// messagescan.Policy. A nil Form scans with the defaults.
func (f *Form) ScanningEnabled() bool { return f == nil || f.ScanEnabled }

func (f *Form) ScanThresholds() (flag, quarantine, reject float64) {
	if f == nil {
		return 0, 0, 0
	}
	return f.FlagThreshold, f.QuarantineThreshold, f.RejectThreshold
}

func (f *Form) ScanSource() ScanSource {
	if f == nil {
		return ScanSource{}
	}
	return ScanSource{Kind: ScanSourceForm, ID: f.ID}
}

// OriginPolicy returns the origin allowlist and whether a missing Origin is
// refused, for messages.OriginAllowed and RedirectAllowed.
func (f *Form) OriginPolicy() ([]string, bool) { return f.AllowedOrigins, f.StrictOrigin }

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
