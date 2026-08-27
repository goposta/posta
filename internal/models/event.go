// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

type EventCategory string

const (
	EventCategoryUser   EventCategory = "user"
	EventCategoryEmail  EventCategory = "email"
	EventCategorySystem EventCategory = "system"
	EventCategoryAudit  EventCategory = "audit"
)

type Event struct {
	ID          uint          `json:"id" gorm:"primaryKey"`
	Category    EventCategory `json:"category" gorm:"index;not null"`
	Type        string        `json:"type" gorm:"index;not null"`
	WorkspaceID *uint         `json:"workspace_id,omitempty" gorm:"index"`
	ActorID     *uint         `json:"actor_id" gorm:"index"`
	ActorName   string        `json:"actor_name"`
	ClientIP    string        `json:"client_ip,omitempty" gorm:"size:45"`
	Message     string        `json:"message" gorm:"not null"`
	Metadata    string        `json:"metadata" gorm:"type:text"`
	CreatedAt   time.Time     `json:"created_at" gorm:"index"`
}
