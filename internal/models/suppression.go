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

// SuppressionKind classifies why an address is suppressed. It makes the global
// vs list-scoped distinction explicit (rather than inferring it from list_id)
// and powers per-reason analytics and a future preference center.
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
	// ListID nil = global block (bounces, complaints, manual, "unsubscribe from
	// everything"); set = opt-out of that UnsubscribeList only.
	ListID    *uint           `json:"list_id,omitempty" gorm:"index"`
	Kind      SuppressionKind `json:"kind" gorm:"type:varchar(20);default:'hard';not null"`
	Reason    string          `json:"reason"`
	CreatedAt time.Time       `json:"created_at"`

	User User `json:"-" gorm:"foreignKey:UserID"`
}
