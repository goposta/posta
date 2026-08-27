// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

// UserSetting stores per-user preferences. Each user has at most one row.
type UserSetting struct {
	ID                 uint   `json:"id" gorm:"primaryKey"`
	UserID             uint   `json:"user_id" gorm:"uniqueIndex;not null"`
	Timezone           string `json:"timezone" gorm:"default:UTC"`
	DefaultSenderName  string `json:"default_sender_name"`
	DefaultSenderEmail string `json:"default_sender_email"`
	EmailNotifications bool   `json:"email_notifications" gorm:"default:true"`
	NotificationEmail  string `json:"notification_email"`
	WebhookRetryCount  int    `json:"webhook_retry_count" gorm:"default:3"`
	DefaultTemplateID  *uint  `json:"default_template_id"`
	APIKeyExpiryDays   int    `json:"api_key_expiry_days" gorm:"default:90"`
	BounceAutoSuppress bool   `json:"bounce_auto_suppress" gorm:"default:true"`
	DefaultLanguage    string `json:"default_language" gorm:"size:10;default:'en'"`
	DailyReport        bool   `json:"daily_report" gorm:"default:true"`

	NotifyBounceAlerts      bool `json:"notify_bounce_alerts" gorm:"default:true"`
	NotifyAPIKeyExpiry      bool `json:"notify_api_key_expiry" gorm:"default:true"`
	NotifyWorkspaceActivity bool `json:"notify_workspace_activity" gorm:"default:true"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
