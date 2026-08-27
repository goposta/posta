// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import (
	"time"

	"github.com/lib/pq"
)

type Webhook struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	UserID      uint           `json:"user_id" gorm:"index;not null"`
	WorkspaceID *uint          `json:"workspace_id,omitempty" gorm:"index"`
	URL         string         `json:"url" gorm:"not null"`
	Events      pq.StringArray `json:"events" gorm:"type:text[];not null"`
	Filters     pq.StringArray `json:"filters" gorm:"type:text[]"`
	Secret      string         `json:"secret,omitempty" gorm:"type:varchar(64);not null;default:''"`
	CreatedAt   time.Time      `json:"created_at"`

	User User `json:"-" gorm:"foreignKey:UserID"`
}
