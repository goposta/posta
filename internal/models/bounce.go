// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

type BounceType string

const (
	BounceTypeHard      BounceType = "hard"
	BounceTypeSoft      BounceType = "soft"
	BounceTypeComplaint BounceType = "complaint"
)

type Bounce struct {
	ID          uint       `json:"id" gorm:"primaryKey"`
	UserID      uint       `json:"user_id" gorm:"index;not null"`
	WorkspaceID *uint      `json:"workspace_id,omitempty" gorm:"index"`
	EmailID     uint       `json:"email_id" gorm:"index;not null"`
	Recipient   string     `json:"recipient" gorm:"not null"`
	Type        BounceType `json:"type" gorm:"not null"`
	Reason      string     `json:"reason"`
	CreatedAt   time.Time  `json:"created_at"`

	User  User  `json:"-" gorm:"foreignKey:UserID"`
	Email Email `json:"-" gorm:"foreignKey:EmailID"`
}
