/*
 * Copyright 2026 Jonas Kaninda
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

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
