// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

// WorkspaceSSOConfig controls SSO enforcement for a workspace.
type WorkspaceSSOConfig struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	WorkspaceID    uint      `json:"workspace_id" gorm:"uniqueIndex;not null"`
	ProviderID     uint      `json:"provider_id" gorm:"not null"`
	EnforceSSO     bool      `json:"enforce_sso" gorm:"default:false"`
	AutoProvision  bool      `json:"auto_provision" gorm:"default:true"`
	AllowedDomains string    `json:"allowed_domains"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`

	Workspace Workspace     `json:"-" gorm:"foreignKey:WorkspaceID"`
	Provider  OAuthProvider `json:"-" gorm:"foreignKey:ProviderID"`
}
