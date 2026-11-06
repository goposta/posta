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

type TrackingEventType string

const (
	TrackingEventOpen        TrackingEventType = "open"
	TrackingEventClick       TrackingEventType = "click"
	TrackingEventUnsubscribe TrackingEventType = "unsubscribe"
)

type TrackedLink struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	CampaignID  uint      `json:"campaign_id" gorm:"index;not null"`
	OriginalURL string    `json:"original_url" gorm:"type:text;not null"`
	Hash        string    `json:"hash" gorm:"uniqueIndex;not null"`
	ClickCount  int64     `json:"click_count" gorm:"default:0;not null"`
	CreatedAt   time.Time `json:"created_at"`

	Campaign Campaign `json:"-" gorm:"foreignKey:CampaignID"`
}

type TrackingEvent struct {
	ID                uint              `json:"id" gorm:"primaryKey"`
	CampaignMessageID uint              `json:"campaign_message_id" gorm:"index;not null"`
	EventType         TrackingEventType `json:"event_type" gorm:"type:varchar(20);not null;index"`
	TrackedLinkID     *uint             `json:"tracked_link_id,omitempty" gorm:"index"`
	IP                string            `json:"ip" gorm:"size:45"`
	UserAgent         string            `json:"user_agent" gorm:"type:text"`
	CreatedAt         time.Time         `json:"created_at" gorm:"index"`

	CampaignMessage CampaignMessage `json:"-" gorm:"foreignKey:CampaignMessageID"`
	TrackedLink     *TrackedLink    `json:"-" gorm:"foreignKey:TrackedLinkID"`
}
