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

// UnsubscribeList is a transactional opt-out scope. It is email-keyed (decoupled
// from Subscriber) and carries no membership: a recipient is never enrolled, the
// list is purely a suppression scope referenced by a send. A one-click on a link
// minted for an email that names a list writes a list-scoped Suppression, so the
// recipient's other transactional mail (receipts, password resets) keeps flowing.
//
// It is distinct from SubscriberList (campaign audiences, subscriber-keyed) and
// its SubscriberListUnsubscribe opt-out table.
type UnsubscribeList struct {
	ID uint `json:"id" gorm:"primaryKey"`
	// UUID is the opaque, non-enumerable public handle (for hosted pages / webhook
	// payloads). The authenticated API still references the list by id.
	UUID        string `json:"uuid" gorm:"type:uuid;default:gen_random_uuid();uniqueIndex;not null"`
	UserID      uint   `json:"user_id" gorm:"index;not null"`
	WorkspaceID *uint  `json:"workspace_id,omitempty" gorm:"index"`
	Name        string `json:"name" gorm:"not null"` // unique per scope (internal label)
	// PublicName is shown to recipients on the unsubscribe/preference page. Falls
	// back to Name when empty.
	PublicName  string     `json:"public_name,omitempty"`
	Description string     `json:"description"`
	Active      bool       `json:"active" gorm:"default:true;not null"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at"`

	User User `json:"-" gorm:"foreignKey:UserID"`
}
