// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

// Session tracks an active JWT session for a user.
type Session struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    uint      `json:"user_id" gorm:"index;not null"`
	JTI       string    `json:"jti" gorm:"uniqueIndex;not null;size:36"` // JWT ID (UUID)
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	Revoked   bool      `json:"revoked" gorm:"default:false;not null"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`

	User User `json:"-" gorm:"foreignKey:UserID"`
}

// IsExpired returns true if the session has passed its expiry time.
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// IsActive returns true if the session is neither revoked nor expired.
func (s *Session) IsActive() bool {
	return !s.Revoked && !s.IsExpired()
}
