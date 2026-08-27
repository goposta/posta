// SPDX-FileCopyrightText: 2026 Jonas Kaninda
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
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}
