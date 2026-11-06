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

// A value of 0 for any Plan limit field means unlimited.
type Plan struct {
	ID                  uint   `json:"id" gorm:"primaryKey"`
	Name                string `json:"name" gorm:"uniqueIndex;not null"`
	Description         string `json:"description"`
	IsDefault           bool   `json:"is_default" gorm:"default:false"`
	IsActive            bool   `json:"is_active" gorm:"default:true"`
	DailyRateLimit      int    `json:"daily_rate_limit" gorm:"default:0"`
	HourlyRateLimit     int    `json:"hourly_rate_limit" gorm:"default:0"`
	MaxAttachmentSizeMB int    `json:"max_attachment_size_mb" gorm:"default:0"`
	MaxBatchSize        int    `json:"max_batch_size" gorm:"default:0"`
	MaxAPIKeys          int    `json:"max_api_keys" gorm:"default:0"`
	MaxDomains          int    `json:"max_domains" gorm:"default:0"`
	MaxSMTPServers      int    `json:"max_smtp_servers" gorm:"default:0"`
	// Live chat. Enforced only under POSTA_PLAN_ENFORCEMENT; 0 is unlimited.
	MaxChatWidgets           int       `json:"max_chat_widgets" gorm:"default:0"`
	MaxChatAgents            int       `json:"max_chat_agents" gorm:"default:0"`
	MonthlyConversationLimit int       `json:"monthly_conversation_limit" gorm:"default:0"`
	MaxWorkspaces            int       `json:"max_workspaces" gorm:"default:0"`
	EmailLogRetentionDays    int       `json:"email_log_retention_days" gorm:"default:0"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
}
