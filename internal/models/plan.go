// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

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
