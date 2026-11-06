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
)

type EmailStatus string

const (
	EmailStatusPending    EmailStatus = "pending"
	EmailStatusQueued     EmailStatus = "queued"
	EmailStatusProcessing EmailStatus = "processing"
	EmailStatusSent       EmailStatus = "sent"
	EmailStatusFailed     EmailStatus = "failed"
	EmailStatusSuppressed EmailStatus = "suppressed"
	EmailStatusScheduled  EmailStatus = "scheduled"
)

type Email struct {
	ID                    uint           `json:"id" gorm:"primaryKey"`
	UUID                  string         `json:"uuid" gorm:"type:uuid;default:gen_random_uuid();uniqueIndex;not null"`
	UserID                uint           `json:"user_id" gorm:"index;not null"`
	WorkspaceID           *uint          `json:"workspace_id,omitempty" gorm:"index"`
	APIKeyID              *uint          `json:"api_key_id" gorm:"index"`
	Sender                string         `json:"sender" gorm:"not null"`
	Recipients            pq.StringArray `json:"recipients" gorm:"type:text[];not null"`
	Subject               string         `json:"subject" gorm:"not null"`
	TemplateName          string         `json:"template_name,omitempty"`
	HTMLBody              string         `json:"html_body"`
	TextBody              string         `json:"text_body"`
	AttachmentsJSON       string         `json:"attachments_json,omitempty" gorm:"type:text"`
	HeadersJSON           string         `json:"headers_json,omitempty" gorm:"type:text"`
	ListUnsubscribeURL    string         `json:"list_unsubscribe_url,omitempty" gorm:"type:text"`
	ListUnsubscribeMailto string         `json:"list_unsubscribe_mailto,omitempty" gorm:"type:text"`
	ListUnsubscribePost   bool           `json:"list_unsubscribe_post,omitempty"`
	UnsubscribeListID     *uint          `json:"unsubscribe_list_id,omitempty" gorm:"index"`
	Status                EmailStatus    `json:"status" gorm:"default:pending;not null"`
	ErrorMessage          string         `json:"error_message"`
	RetryCount            int            `json:"retry_count" gorm:"default:0;not null"`
	CreatedAt             time.Time      `json:"created_at"`
	SentAt                *time.Time     `json:"sent_at"`
	ScheduledAt           *time.Time     `json:"scheduled_at"`
	Provider              string         `json:"provider,omitempty" gorm:"index;size:32"`

	SMTPHostname string `json:"smtp_hostname,omitempty"`

	User   User   `json:"-" gorm:"foreignKey:UserID"`
	APIKey APIKey `json:"-" gorm:"foreignKey:APIKeyID;constraint:OnDelete:SET NULL"`

	APIKeyName string `json:"api_key_name,omitempty" gorm:"-"`
}

type Attachment struct {
	Filename    string `json:"filename"`
	Content     string `json:"content,omitempty"`
	ContentType string `json:"content_type"`
	// StorageKey, when set, means the bytes live in blob storage and Content may be empty.
	StorageKey string `json:"storage_key,omitempty"`
}
