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

import (
	"time"

	"github.com/lib/pq"
)

// API key scopes gate what an API-key caller may do inside the workspace the key
// is scoped to. They never confer platform administration: /api/v1/admin/* is
// reachable only from the admin dashboard with a user session.
const (
	ScopeSend     = "send"     // send emails, batches, templates; subscriber-list ops
	ScopeRead     = "read"     // read-only access to workspace resources
	ScopeWebhooks = "webhooks" // create, list, and delete webhooks
	ScopeWrite    = "write"    // create/update/delete workspace resources
	ScopeAdmin    = "admin"    // administrative operations within the workspace
	ScopeSetup    = "setup"    // reserved, not yet accepted on creation
	ScopeAll      = "*"        // all scopes
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
	CreatedBy *ActorRef `json:"created_by,omitempty" gorm:"foreignKey:UserID;references:ID;constraint:false"`
}

// HasScope reports whether the key grants scope s. An empty scope set means the
// key predates scopes and grants only `send`; "*" grants everything.
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
