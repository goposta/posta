// Copyright (c) 2026 Jonas Kaninda.
//
// This file is part of Posta.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

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
