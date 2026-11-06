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

import (
	"time"

	"gorm.io/gorm"
)

// ChatSnippet is a canned reply: agents type "/" and its shortcut in the composer to insert Body.
type ChatSnippet struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	WorkspaceID *uint          `json:"workspace_id,omitempty" gorm:"index;not null"`
	Shortcut    string         `json:"shortcut" gorm:"not null"`
	Title       string         `json:"title" gorm:"not null"`
	Body        string         `json:"body" gorm:"type:text;not null"`
	CreatedByID *uint          `json:"created_by_id,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   *time.Time     `json:"updated_at,omitempty"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}
