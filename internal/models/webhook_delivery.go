// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

// WebhookDeliveryStatus represents the outcome of a webhook delivery attempt.
type WebhookDeliveryStatus string

const (
	WebhookDeliverySuccess WebhookDeliveryStatus = "success"
	WebhookDeliveryFailed  WebhookDeliveryStatus = "failed"
)

// WebhookDelivery records the result of delivering a webhook notification.
type WebhookDelivery struct {
	ID             uint                  `json:"id" gorm:"primaryKey"`
	WebhookID      uint                  `json:"webhook_id" gorm:"index;not null"`
	UserID         uint                  `json:"user_id" gorm:"index;not null"`
	WorkspaceID    *uint                 `json:"workspace_id,omitempty" gorm:"index"`
	Event          string                `json:"event" gorm:"not null"`
	Status         WebhookDeliveryStatus `json:"status" gorm:"type:varchar(16);not null;index"`
	HTTPStatusCode int                   `json:"http_status_code"`
	ErrorMessage   string                `json:"error_message,omitempty"`
	Attempt        int                   `json:"attempt" gorm:"not null;default:1"`
	CreatedAt      time.Time             `json:"created_at"`

	Webhook Webhook `json:"-" gorm:"foreignKey:WebhookID"`
	User    User    `json:"-" gorm:"foreignKey:UserID"`
}
