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

// ChatWidgetAsset is an image shown by a public widget; Data holds it when the server has no blob store.
type ChatWidgetAsset struct {
	ID          uint      `json:"-" gorm:"primaryKey"`
	UUID        string    `json:"id" gorm:"type:uuid;default:gen_random_uuid();uniqueIndex;not null"`
	WorkspaceID *uint     `json:"-" gorm:"index;not null"`
	WidgetID    uint      `json:"-" gorm:"index;not null"`
	Filename    string    `json:"filename" gorm:"not null;default:''"`
	ContentType string    `json:"content_type" gorm:"type:varchar(32);not null"`
	Size        int64     `json:"size" gorm:"not null"`
	Key         string    `json:"-" gorm:"not null;default:''"`
	Data        []byte    `json:"-" gorm:"type:bytea"`
	CreatedAt   time.Time `json:"created_at"`
	URL         string    `json:"url" gorm:"-"`
}
