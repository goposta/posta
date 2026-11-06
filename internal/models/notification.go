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

// NotificationKind separates alerts Posta derives itself from announcements a platform admin broadcasts.
type NotificationKind string

const (
	NotificationKindAlert        NotificationKind = "alert"
	NotificationKindAnnouncement NotificationKind = "announcement"
)

// Only alerts carry warning/critical; announcements are info unless the operator says otherwise.
type NotificationSeverity string

const (
	NotificationInfo     NotificationSeverity = "info"
	NotificationWarning  NotificationSeverity = "warning"
	NotificationCritical NotificationSeverity = "critical"
)

type NotificationCategory string

const (
	NotificationCategoryDomains        NotificationCategory = "domains"
	NotificationCategoryDeliverability NotificationCategory = "deliverability"
	NotificationCategorySecurity       NotificationCategory = "security"
	NotificationCategoryMessages       NotificationCategory = "messages"
	NotificationCategoryPlatform       NotificationCategory = "platform"
)

// Per-user, not per-workspace: read/dismissed state belongs to a person, so workspace alerts fan out per member.
type Notification struct {
	ID     uint `json:"id" gorm:"primaryKey"`
	UserID uint `json:"user_id" gorm:"not null;index:idx_notification_user_open,priority:1"`
	// WorkspaceID is nil for platform-wide items, which follow the user rather than any one workspace.
	WorkspaceID *uint                `json:"workspace_id,omitempty" gorm:"index"`
	Kind        NotificationKind     `json:"kind" gorm:"not null;default:alert"`
	Category    NotificationCategory `json:"category" gorm:"not null"`
	Severity    NotificationSeverity `json:"severity" gorm:"not null;default:info"`

	Title      string `json:"title" gorm:"not null"`
	Body       string `json:"body"`
	Link       string `json:"link,omitempty"`
	ActionText string `json:"action_text,omitempty"`

	// DedupKey names the condition, not the occurrence: one open row per (user, workspace, key) updates in place.
	DedupKey string `json:"dedup_key" gorm:"not null;index:idx_notification_user_open,priority:2"`

	// Dismissal binds to the Fingerprint, so dismissing "3 unverified" resurfaces at 4 instead of silencing forever.
	Fingerprint string `json:"-" gorm:"not null;default:''"`

	ReadAt      *time.Time `json:"read_at,omitempty"`
	DismissedAt *time.Time `json:"dismissed_at,omitempty"`
	// Resolved items stay in the inbox as history but leave the dashboard banner.
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`

	CreatedAt time.Time `json:"created_at" gorm:"index"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Open reports a live, undismissed condition: the set the dashboard banner renders.
func (n *Notification) Open() bool {
	return n.ResolvedAt == nil && n.DismissedAt == nil
}

// Announcement is its own row so the operator can see, edit or retract it apart from per-user deliveries.
type Announcement struct {
	ID    uint   `json:"id" gorm:"primaryKey"`
	Title string `json:"title" gorm:"not null"`
	// Not named Body: okapi silently binds a Body field as the request body. See TestRequestBodiesHaveNoBodyField.
	Message    string               `json:"message"`
	Link       string               `json:"link,omitempty"`
	Severity   NotificationSeverity `json:"severity" gorm:"not null;default:info"`
	CreatedBy  uint                 `json:"created_by" gorm:"not null"`
	AuthorName string               `json:"author_name"`
	// Recipients is how many inbox rows the broadcast produced.
	Recipients int        `json:"recipients" gorm:"not null;default:0"`
	SentAt     *time.Time `json:"sent_at,omitempty"`

	CreatedAt time.Time `json:"created_at" gorm:"index"`
	UpdatedAt time.Time `json:"updated_at"`
}
