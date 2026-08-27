// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

type StyleSheet struct {
	ID          uint       `json:"id" gorm:"primaryKey"`
	UserID      uint       `json:"user_id" gorm:"index;not null"`
	WorkspaceID *uint      `json:"workspace_id,omitempty" gorm:"index"`
	Name        string     `json:"name" gorm:"not null"`
	CSS         string     `json:"css"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`

	User User `json:"-" gorm:"foreignKey:UserID"`
}
