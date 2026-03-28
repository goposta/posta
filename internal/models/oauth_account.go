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
