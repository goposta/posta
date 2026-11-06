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

// SuppressionKind makes global vs list-scoped explicit instead of inferring it from list_id.
type SuppressionKind string

const (
	SuppressionKindHard            SuppressionKind = "hard"             // manual/automated "block everything"
	SuppressionKindBounce          SuppressionKind = "bounce"           // hard bounce
	SuppressionKindComplaint       SuppressionKind = "complaint"        // spam/abuse complaint
	SuppressionKindListUnsubscribe SuppressionKind = "list_unsubscribe" // opted out of one list
	SuppressionKindManual          SuppressionKind = "manual"           // created via the management API
)

type Suppression struct {
	ID          uint   `json:"id" gorm:"primaryKey"`
	UserID      uint   `json:"user_id" gorm:"index;not null"`
	WorkspaceID *uint  `json:"workspace_id,omitempty" gorm:"index"`
	Email       string `json:"email" gorm:"not null"`
	// ListID nil is a global block; set, it opts out of that UnsubscribeList only.
	ListID    *uint           `json:"list_id,omitempty" gorm:"index"`
	Kind      SuppressionKind `json:"kind" gorm:"type:varchar(20);default:'hard';not null"`
	Reason    string          `json:"reason"`
	CreatedAt time.Time       `json:"created_at"`

	User User `json:"-" gorm:"foreignKey:UserID"`
}
