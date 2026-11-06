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
