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

type Template struct {
	ID              uint       `json:"id" gorm:"primaryKey"`
	UserID          uint       `json:"user_id" gorm:"index;not null"`
	WorkspaceID     *uint      `json:"workspace_id,omitempty" gorm:"index"`
	Name            string     `json:"name" gorm:"not null"`
	DisplayName     string     `json:"display_name" gorm:"not null;default:''"`
	DefaultLanguage string     `json:"default_language" gorm:"default:en;not null"`
	ActiveVersionID *uint      `json:"active_version_id,omitempty" gorm:"uniqueIndex"`
	Description     string     `json:"description"`
	SampleData      string     `json:"sample_data"`
	LastEditedByID  *uint      `json:"last_edited_by_id,omitempty" gorm:"index"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       *time.Time `json:"updated_at,omitempty"`

	User          User             `json:"-" gorm:"foreignKey:UserID"`
	ActiveVersion *TemplateVersion `json:"active_version,omitempty" gorm:"foreignKey:ActiveVersionID;constraint:false"`
	CreatedBy     *ActorRef        `json:"created_by,omitempty" gorm:"->;foreignKey:UserID;references:ID;constraint:false"`
	LastEditedBy  *ActorRef        `json:"last_edited_by,omitempty" gorm:"->;foreignKey:LastEditedByID;references:ID;constraint:false"`
}
