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

type OAuthProviderType string

const (
	OAuthProviderGoogle OAuthProviderType = "google"
	OAuthProviderOIDC   OAuthProviderType = "oidc"
)

// OAuthProvider stores configuration for an OAuth/OIDC identity provider.
type OAuthProvider struct {
	ID             uint              `json:"id" gorm:"primaryKey"`
	WorkspaceID    *uint             `json:"workspace_id,omitempty" gorm:"index"`
	Name           string            `json:"name" gorm:"not null"`
	Slug           string            `json:"slug" gorm:"uniqueIndex;not null"`
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
