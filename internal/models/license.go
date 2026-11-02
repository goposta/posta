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

// License is the installed Posta Enterprise license. There is at most one
// row. Token is authoritative: it is verified again on every load, and the
// other columns are copies kept for operators reading the database.
type License struct {
	ID        uint      `gorm:"primaryKey"`
	LicenseID string    `gorm:"size:64"`
	Customer  string    `gorm:"size:255"`
	Token     string    `gorm:"type:text;not null"`
	NotAfter  time.Time `gorm:"not null"`
	CreatedAt time.Time
}
