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

type InboundEmailStatus string

const (
	InboundStatusReceived  InboundEmailStatus = "received"
	InboundStatusForwarded InboundEmailStatus = "forwarded"
	InboundStatusFailed    InboundEmailStatus = "failed"
	InboundStatusRejected  InboundEmailStatus = "rejected"
)

type InboundSource string

const (
	InboundSourceSMTP    InboundSource = "smtp"
	InboundSourceWebhook InboundSource = "webhook"
)

type InboundEmail struct {
	ID              uint               `json:"id" gorm:"primaryKey"`
	UUID            string             `json:"uuid" gorm:"type:uuid;default:gen_random_uuid();uniqueIndex;not null"`
	UserID          uint               `json:"user_id" gorm:"index;not null"`
	WorkspaceID     *uint              `json:"workspace_id,omitempty" gorm:"index"`
	DomainID        uint               `json:"domain_id" gorm:"index;not null"`
	MessageID       string             `json:"message_id" gorm:"index"`
	Sender          string             `json:"sender" gorm:"not null"`
	Recipients      pq.StringArray     `json:"recipients" gorm:"type:text[];not null"`
	Subject         string             `json:"subject"`
	TextBody        string             `json:"text_body" gorm:"type:text"`
	HTMLBody        string             `json:"html_body" gorm:"type:text"`
	AttachmentsJSON string             `json:"attachments_json,omitempty" gorm:"type:text"`
	HeadersJSON     string             `json:"headers_json,omitempty" gorm:"type:text"`
	RawStorageKey   string             `json:"raw_storage_key,omitempty"`
	DedupHash       string             `json:"-" gorm:"index"`
	Size            int64              `json:"size"`
	SpamScore       *float64           `json:"spam_score,omitempty"`
	Status          InboundEmailStatus `json:"status" gorm:"default:received;not null"`
	Source          InboundSource      `json:"source" gorm:"not null"`
	ErrorMessage    string             `json:"error_message,omitempty"`
	RetryCount      int                `json:"retry_count" gorm:"default:0;not null"`
	ReceivedAt      time.Time          `json:"received_at" gorm:"not null"`
	ForwardedAt     *time.Time         `json:"forwarded_at,omitempty"`
	CreatedAt       time.Time          `json:"created_at"`

	User   User   `json:"-" gorm:"foreignKey:UserID"`
	Domain Domain `json:"-" gorm:"foreignKey:DomainID"`
}

// InboundAttachmentMeta describes one inbound attachment stored in blob storage.
// Serialized into InboundEmail.AttachmentsJSON.
type InboundAttachmentMeta struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	StorageKey  string `json:"storage_key,omitempty"`
	// Content is only populated when no blob store is configured (dev fallback).
	Content string `json:"content,omitempty"`
}
