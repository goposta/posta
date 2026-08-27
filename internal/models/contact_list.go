// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

type ContactList struct {
	ID          uint       `json:"id" gorm:"primaryKey"`
	UserID      uint       `json:"user_id" gorm:"index;not null"`
	WorkspaceID *uint      `json:"workspace_id,omitempty" gorm:"index"`
	Name        string     `json:"name" gorm:"not null"`
	Description string     `json:"description"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at"`

	User    User                `json:"-" gorm:"foreignKey:UserID"`
	Members []ContactListMember `json:"-" gorm:"foreignKey:ListID"`
}

type ContactListMember struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	ListID    uint      `json:"list_id" gorm:"uniqueIndex:idx_list_email;not null"`
	Email     string    `json:"email" gorm:"uniqueIndex:idx_list_email;not null"`
	Name      string    `json:"name"`
	Data      string    `json:"data" gorm:"type:text"` // JSON metadata
	CreatedAt time.Time `json:"created_at"`

	List ContactList `json:"-" gorm:"foreignKey:ListID"`
}
