/*
 * Copyright 2026 Jonas Kaninda
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

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
	ID                  uint           `json:"id" gorm:"primaryKey"`
	UUID                string         `json:"uuid" gorm:"type:uuid;default:gen_random_uuid();uniqueIndex;not null"`
	UserID              uint           `json:"user_id" gorm:"index;not null"`
	APIKeyID            *uint          `json:"api_key_id" gorm:"index"`
	Sender              string         `json:"sender" gorm:"not null"`
	Recipients          pq.StringArray `json:"recipients" gorm:"type:text[];not null"`
	Subject             string         `json:"subject" gorm:"not null"`
	HTMLBody            string         `json:"html_body"`
	TextBody            string         `json:"text_body"`
	AttachmentsJSON     string         `json:"attachments_json,omitempty" gorm:"type:text"`
	HeadersJSON         string         `json:"headers_json,omitempty" gorm:"type:text"`
	ListUnsubscribeURL  string         `json:"list_unsubscribe_url,omitempty" gorm:"type:text"`
	ListUnsubscribePost bool           `json:"list_unsubscribe_post,omitempty"`
	Status              EmailStatus    `json:"status" gorm:"default:pending;not null"`
	ErrorMessage        string         `json:"error_message"`
	RetryCount          int            `json:"retry_count" gorm:"default:0;not null"`
	CreatedAt           time.Time      `json:"created_at"`
	SentAt              *time.Time     `json:"sent_at"`
	ScheduledAt         *time.Time     `json:"scheduled_at"`

	SMTPHostname string `json:"smtp_hostname,omitempty"`

	User   User   `json:"-" gorm:"foreignKey:UserID"`
	APIKey APIKey `json:"-" gorm:"foreignKey:APIKeyID"`
}

// Attachment represents a file attachment in an email.
type Attachment struct {
	Filename    string `json:"filename"`
	Content     string `json:"content"`
	ContentType string `json:"content_type"`
}
