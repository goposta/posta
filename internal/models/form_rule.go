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
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

type RuleMatch string

const (
	RuleMatchAll RuleMatch = "all"
	RuleMatchAny RuleMatch = "any"
)

// RuleCondition tests one answer or a $-prefixed fact: $origin, $referer, $sender_email, $spam_score, $status.
type RuleCondition struct {
	Field string `json:"field"`
	Op    string `json:"op"`
	Value any    `json:"value,omitempty"`
}

// No rule action reads a destination from submission data: every target is chosen by an admin.
const (
	ActionNotifyEmails    = "notify_emails"
	ActionNotifyMembers   = "notify_members"
	ActionNotifyTeam      = "notify_team"
	ActionAssignMember    = "assign_member"
	ActionAssignTeam      = "assign_team"
	ActionAddTags         = "add_tags"
	ActionSetPriority     = "set_priority"
	ActionWebhook         = "webhook"
	ActionSuppressDefault = "suppress_default"
)

type RuleAction struct {
	Type      string   `json:"type"`
	Emails    []string `json:"emails,omitempty"`
	UserIDs   []uint   `json:"user_ids,omitempty"`
	TeamID    *uint    `json:"team_id,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	Priority  string   `json:"priority,omitempty"`
	WebhookID *uint    `json:"webhook_id,omitempty"`
}

type RuleConditions []RuleCondition

func (r RuleConditions) Value() (driver.Value, error) {
	if r == nil {
		return "[]", nil
	}
	return json.Marshal(r)
}

func (r *RuleConditions) Scan(value any) error {
	raw, err := jsonBytes(value)
	if err != nil || raw == nil {
		*r = nil
		return err
	}
	return json.Unmarshal(raw, r)
}

type RuleActions []RuleAction

func (r RuleActions) Value() (driver.Value, error) {
	if r == nil {
		return "[]", nil
	}
	return json.Marshal(r)
}

func (r *RuleActions) Scan(value any) error {
	raw, err := jsonBytes(value)
	if err != nil || raw == nil {
		*r = nil
		return err
	}
	return json.Unmarshal(raw, r)
}

// Rules run in Position order and every match applies, unless a matching rule has Stop set.
type FormRule struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	UUID        string         `json:"uuid" gorm:"type:uuid;default:gen_random_uuid();uniqueIndex;not null"`
	WorkspaceID *uint          `json:"workspace_id,omitempty" gorm:"index;not null"`
	FormID      uint           `json:"form_id" gorm:"index;not null"`
	Name        string         `json:"name" gorm:"not null"`
	Position    int            `json:"position" gorm:"not null;default:0"`
	Enabled     bool           `json:"enabled" gorm:"default:true;not null"`
	Match       RuleMatch      `json:"match" gorm:"type:varchar(8);default:'all';not null"`
	Conditions  RuleConditions `json:"conditions" gorm:"type:jsonb;not null"`
	Actions     RuleActions    `json:"actions" gorm:"type:jsonb;not null"`
	Stop        bool           `json:"stop" gorm:"default:false;not null"`

	HitCount       int64      `json:"hit_count" gorm:"default:0;not null"`
	LastHitAt      *time.Time `json:"last_hit_at,omitempty"`
	LastEditedByID *uint      `json:"last_edited_by_id,omitempty"`

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`

	Form *Form `json:"-" gorm:"foreignKey:FormID;constraint:OnDelete:CASCADE"`
}

// Team is a named group of workspace members that rules notify or assign to, with an optional shared address.
type Team struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	UUID        string         `json:"uuid" gorm:"type:uuid;default:gen_random_uuid();uniqueIndex;not null"`
	WorkspaceID *uint          `json:"workspace_id,omitempty" gorm:"index;not null"`
	Name        string         `json:"name" gorm:"not null"`
	Email       string         `json:"email"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   *time.Time     `json:"updated_at,omitempty"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`

	Members []TeamMember `json:"members,omitempty" gorm:"foreignKey:TeamID;constraint:OnDelete:CASCADE"`
}

// Removing someone from the workspace removes their team memberships with it.
type TeamMember struct {
	TeamID uint      `json:"team_id" gorm:"primaryKey"`
	UserID uint      `json:"user_id" gorm:"primaryKey"`
	User   *ActorRef `json:"user,omitempty" gorm:"->;foreignKey:UserID;references:ID;constraint:false"`
}

type TeamRef struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

func (TeamRef) TableName() string { return "teams" }

type RuleRef struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

type RuleUUIDs = pq.StringArray
