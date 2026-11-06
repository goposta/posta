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

// Email-keyed opt-out scope with no membership; unsubscribing suppresses only this list, so other mail keeps flowing.
type UnsubscribeList struct {
	ID uint `json:"id" gorm:"primaryKey"`
	// UUID is the non-enumerable public handle for hosted pages and webhooks; the authenticated API uses the id.
	UUID        string `json:"uuid" gorm:"type:uuid;default:gen_random_uuid();uniqueIndex;not null"`
	UserID      uint   `json:"user_id" gorm:"index;not null"`
	WorkspaceID *uint  `json:"workspace_id,omitempty" gorm:"index"`
	Name        string `json:"name" gorm:"not null"` // unique per scope (internal label)
	// PublicName is shown to recipients on the unsubscribe page; falls back to Name when empty.
	PublicName  string     `json:"public_name,omitempty"`
	Description string     `json:"description"`
	Active      bool       `json:"active" gorm:"default:true;not null"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at"`

	User User `json:"-" gorm:"foreignKey:UserID"`
}
