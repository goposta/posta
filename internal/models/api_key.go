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

	"github.com/lib/pq"
)

// Scopes gate API-key access inside its workspace and never confer platform administration (/api/v1/admin/*).
const (
	ScopeSend     = "send"     // send emails, batches, templates; subscriber-list ops
	ScopeRead     = "read"     // read-only access to workspace resources
	ScopeWebhooks = "webhooks" // create, list, and delete webhooks
	ScopeWrite    = "write"    // create/update/delete workspace resources
	ScopeAdmin    = "admin"    // administrative operations within the workspace
	ScopeSetup    = "setup"    // reserved, not yet accepted on creation
	ScopeAll      = "*"
)

var ValidScopes = map[string]bool{
	ScopeSend:     true,
	ScopeRead:     true,
	ScopeWebhooks: true,
	ScopeWrite:    true,
	ScopeAdmin:    true,
	ScopeAll:      true,
}

type APIKey struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	UserID      uint           `json:"user_id" gorm:"index;not null"`
	WorkspaceID *uint          `json:"workspace_id,omitempty" gorm:"index"`
	Name        string         `json:"name" gorm:"not null"`
	KeyHash     string         `json:"-" gorm:"not null"`
	KeyPrefix   string         `json:"key_prefix" gorm:"not null"`
	CreatedAt   time.Time      `json:"created_at"`
	ExpiresAt   *time.Time     `json:"expires_at"`
	LastUsedAt  *time.Time     `json:"last_used_at"`
	Revoked     bool           `json:"revoked" gorm:"default:false"`
	AllowedIPs  pq.StringArray `json:"allowed_ips" gorm:"type:text[]"`
	Scopes      pq.StringArray `json:"scopes" gorm:"type:text[]"`

	User      User      `json:"-" gorm:"foreignKey:UserID"`
	CreatedBy *ActorRef `json:"created_by,omitempty" gorm:"->;foreignKey:UserID;references:ID;constraint:false"`
}

// HasScope treats an empty scope set as a pre-scopes key granting only `send`; "*" grants everything.
func (k *APIKey) HasScope(s string) bool {
	if len(k.Scopes) == 0 {
		return s == ScopeSend
	}
	for _, sc := range k.Scopes {
		if sc == ScopeAll || sc == s {
			return true
		}
	}
	return false
}

func (k *APIKey) IsExpired() bool {
	if k.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*k.ExpiresAt)
}

func (k *APIKey) IsValid() bool {
	return !k.Revoked && !k.IsExpired()
}
