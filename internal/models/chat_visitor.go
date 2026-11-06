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
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// ChatVisitor is an anonymous browser that has written to a chat widget. It
// is created with the visitor's first message, not when the page loads, and
// is known by a bearer token whose hash is all Posta keeps.
//
// Name and Email are whatever the visitor typed. They are shown to agents and
// never used to match the visitor to anyone else's history.
type ChatVisitor struct {
	ID          uint   `json:"id" gorm:"primaryKey"`
	UUID        string `json:"uuid" gorm:"type:uuid;default:gen_random_uuid();uniqueIndex;not null"`
	WorkspaceID *uint  `json:"workspace_id,omitempty" gorm:"index;not null"`
	WidgetID    uint   `json:"widget_id" gorm:"index;not null"`
	TokenHash   string `json:"-" gorm:"uniqueIndex;not null"`

	Name  string `json:"name"`
	Email string `json:"email"`
	// EmailVerified is set only by a signed identity from the customer's
	// server, never by what the visitor typed. ExternalID is the customer's
	// user ID from that identity; visitors sharing a verified ExternalID on a
	// widget share their history.
	EmailVerified bool           `json:"email_verified" gorm:"default:false;not null"`
	ExternalID    string         `json:"external_id,omitempty" gorm:"index"`
	Attributes    ChatAttributes `json:"attributes,omitempty" gorm:"type:jsonb"`
	BannedAt      *time.Time     `json:"banned_at,omitempty" gorm:"index"`

	FirstSeenIP string    `json:"first_seen_ip" gorm:"size:45"`
	LastSeenIP  string    `json:"last_seen_ip" gorm:"size:45"`
	UserAgent   string    `json:"user_agent"`
	LastPageURL string    `json:"last_page_url"`
	LastSeenAt  time.Time `json:"last_seen_at"`
	CreatedAt   time.Time `json:"created_at"`
}

// ChatAttributes are custom facts the customer's page sets on a visitor
// (plan, account age): flat string, number and boolean values, 2 KiB at most.
type ChatAttributes map[string]any

// MaxChatAttributesBytes caps a visitor's attributes once encoded.
const MaxChatAttributesBytes = 2048

func (a ChatAttributes) Value() (driver.Value, error) {
	if len(a) == 0 {
		return nil, nil
	}
	return json.Marshal(a)
}

func (a *ChatAttributes) Scan(value any) error {
	*a = nil
	switch v := value.(type) {
	case nil:
		return nil
	case []byte:
		return json.Unmarshal(v, a)
	case string:
		return json.Unmarshal([]byte(v), a)
	}
	return fmt.Errorf("chat attributes: unsupported type %T", value)
}
