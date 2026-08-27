// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

type Language struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	UserID      uint      `json:"user_id" gorm:"index;not null"`
	WorkspaceID *uint     `json:"workspace_id,omitempty" gorm:"index"`
	Code        string    `json:"code" gorm:"not null;size:10"`
	Name        string    `json:"name" gorm:"not null"`
	IsDefault   bool      `json:"is_default" gorm:"default:false"`
	CreatedAt   time.Time `json:"created_at"`

	User User `json:"-" gorm:"foreignKey:UserID"`
}
