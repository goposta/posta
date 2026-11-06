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

import "time"

type WebhookDeliveryStatus string

const (
	WebhookDeliverySuccess WebhookDeliveryStatus = "success"
	WebhookDeliveryFailed  WebhookDeliveryStatus = "failed"
)

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
