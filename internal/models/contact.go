// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

type Contact struct {
	ID          uint       `json:"id" gorm:"primaryKey"`
	UserID      uint       `json:"user_id" gorm:"index;not null"`
	WorkspaceID *uint      `json:"workspace_id,omitempty" gorm:"index"`
	Email       string     `json:"email" gorm:"not null"`
	Name        string     `json:"name" gorm:"default:''"`
	SentCount   int64      `json:"sent_count" gorm:"default:0;not null"`
	FailCount   int64      `json:"fail_count" gorm:"default:0;not null"`
	LastSentAt  *time.Time `json:"last_sent_at"`
	CreatedAt   time.Time  `json:"created_at"`
}
