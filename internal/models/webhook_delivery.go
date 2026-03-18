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
	Event          string                `json:"event" gorm:"not null"`
	Status         WebhookDeliveryStatus `json:"status" gorm:"type:varchar(16);not null;index"`
	HTTPStatusCode int                   `json:"http_status_code"`
	ErrorMessage   string                `json:"error_message,omitempty"`
	Attempt        int                   `json:"attempt" gorm:"not null;default:1"`
	CreatedAt      time.Time             `json:"created_at"`

	Webhook Webhook `json:"-" gorm:"foreignKey:WebhookID"`
	User    User    `json:"-" gorm:"foreignKey:UserID"`
}
