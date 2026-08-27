// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

type UserEmailVerification struct {
	ID        uint       `gorm:"primaryKey"`
	UserID    uint       `gorm:"index;not null"`
	TokenHash string     `gorm:"uniqueIndex;size:64;not null"`
	ExpiresAt time.Time  `gorm:"not null"`
	UsedAt    *time.Time `gorm:""`
	CreatedAt time.Time  `gorm:"not null"`
}
