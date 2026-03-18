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

// UserSetting stores per-user preferences. Each user has at most one row.
type UserSetting struct {
	ID                 uint      `json:"id" gorm:"primaryKey"`
	UserID             uint      `json:"user_id" gorm:"uniqueIndex;not null"`
	Timezone           string    `json:"timezone" gorm:"default:UTC"`
	DefaultSenderName  string    `json:"default_sender_name"`
	DefaultSenderEmail string    `json:"default_sender_email"`
	EmailNotifications bool      `json:"email_notifications" gorm:"default:true"`
	NotificationEmail  string    `json:"notification_email"`
	WebhookRetryCount  int       `json:"webhook_retry_count" gorm:"default:3"`
	DefaultTemplateID  *uint     `json:"default_template_id"`
	APIKeyExpiryDays   int       `json:"api_key_expiry_days" gorm:"default:90"`
	BounceAutoSuppress bool      `json:"bounce_auto_suppress" gorm:"default:true"`
	DailyReport        bool      `json:"daily_report" gorm:"default:false"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}
