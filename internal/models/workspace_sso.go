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
