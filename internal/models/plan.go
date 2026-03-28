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

// Plan defines a usage package that can be assigned to workspaces.
// Each plan specifies rate limits, resource quotas, and retention policies.
// A value of 0 for any limit field means unlimited.
type Plan struct {
	ID                    uint      `json:"id" gorm:"primaryKey"`
	Name                  string    `json:"name" gorm:"uniqueIndex;not null"`
	Description           string    `json:"description"`
	IsDefault             bool      `json:"is_default" gorm:"default:false"`
	IsActive              bool      `json:"is_active" gorm:"default:true"`
	DailyRateLimit        int       `json:"daily_rate_limit" gorm:"default:0"`
	HourlyRateLimit       int       `json:"hourly_rate_limit" gorm:"default:0"`
	MaxAttachmentSizeMB   int       `json:"max_attachment_size_mb" gorm:"default:0"`
	MaxBatchSize          int       `json:"max_batch_size" gorm:"default:0"`
	MaxAPIKeys            int       `json:"max_api_keys" gorm:"default:0"`
	MaxDomains            int       `json:"max_domains" gorm:"default:0"`
	MaxSMTPServers        int       `json:"max_smtp_servers" gorm:"default:0"`
	MaxWorkspaces         int       `json:"max_workspaces" gorm:"default:0"`
	EmailLogRetentionDays int       `json:"email_log_retention_days" gorm:"default:0"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}
