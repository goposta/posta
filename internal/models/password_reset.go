// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

// PasswordResetToken backs the "forgot password" flow: a single-use, hashed,
// expiring token tied to a user. Only the SHA-256 hash is stored.
type PasswordResetToken struct {
	ID        uint       `gorm:"primaryKey"`
	UserID    uint       `gorm:"index;not null"`
	TokenHash string     `gorm:"uniqueIndex;size:64;not null"`
	ExpiresAt time.Time  `gorm:"not null"`
	UsedAt    *time.Time `gorm:""`
	CreatedAt time.Time  `gorm:"not null"`
}
