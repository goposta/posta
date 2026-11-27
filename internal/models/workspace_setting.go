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

type WorkspaceSetting struct {
	ID                    uint      `json:"id" gorm:"primaryKey"`
	WorkspaceID           uint      `json:"workspace_id" gorm:"uniqueIndex;not null"`
	Timezone              string    `json:"timezone" gorm:"default:UTC"`
	DefaultSenderName     string    `json:"default_sender_name"`
	DefaultSenderEmail    string    `json:"default_sender_email"`
	WebhookRetryCount     int       `json:"webhook_retry_count" gorm:"default:3"`
	APIKeyExpiryDays      int       `json:"api_key_expiry_days" gorm:"default:90"`
	BounceAutoSuppress    bool      `json:"bounce_auto_suppress" gorm:"default:true"`
	RequireVerifiedDomain bool      `json:"require_verified_domain" gorm:"default:false"`
	TrackOpens            bool      `json:"track_opens" gorm:"default:true;not null"`
	TrackClicks           bool      `json:"track_clicks" gorm:"default:true;not null"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}
