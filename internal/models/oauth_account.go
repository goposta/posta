// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

// OAuthAccount links an external OAuth identity to a local user.
type OAuthAccount struct {
	ID             uint       `json:"id" gorm:"primaryKey"`
	UserID         uint       `json:"user_id" gorm:"uniqueIndex:idx_user_provider;not null"`
	ProviderID     uint       `json:"provider_id" gorm:"uniqueIndex:idx_user_provider;not null"`
	ProviderUserID string     `json:"provider_user_id" gorm:"uniqueIndex:idx_provider_ext_id;not null"`
	Email          string     `json:"email"`
	Name           string     `json:"name"`
	AvatarURL      string     `json:"avatar_url"`
	AccessToken    string     `json:"-" gorm:"type:text"`
	RefreshToken   string     `json:"-" gorm:"type:text"`
	TokenExpiresAt *time.Time `json:"-"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`

	User     User          `json:"-" gorm:"foreignKey:UserID"`
	Provider OAuthProvider `json:"-" gorm:"foreignKey:ProviderID"`
}
