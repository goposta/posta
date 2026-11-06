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

type OAuthProviderType string

const (
	OAuthProviderGoogle OAuthProviderType = "google"
	OAuthProviderOIDC   OAuthProviderType = "oidc"
)

type OAuthProvider struct {
	ID             uint              `json:"id" gorm:"primaryKey"`
	WorkspaceID    *uint             `json:"workspace_id,omitempty" gorm:"index"`
	Name           string            `json:"name" gorm:"uniqueIndex;not null"`
	DisplayName    string            `json:"display_name" gorm:"not null;default:''"`
	Type           OAuthProviderType `json:"type" gorm:"not null"`
	ClientID       string            `json:"-" gorm:"not null"`
	ClientSecret   string            `json:"-" gorm:"not null"`
	Issuer         string            `json:"issuer"`
	AuthURL        string            `json:"auth_url"`
	TokenURL       string            `json:"token_url"`
	UserInfoURL    string            `json:"userinfo_url"`
	Scopes         string            `json:"scopes" gorm:"default:'openid email profile'"`
	Enabled        bool              `json:"enabled" gorm:"default:true;not null"`
	Hidden         bool              `json:"hidden" gorm:"default:false;not null"`
	AutoRegister   bool              `json:"auto_register" gorm:"default:true"`
	AllowedDomains string            `json:"allowed_domains"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}
