// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

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
