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

type TrackingEventType string

const (
	TrackingEventOpen        TrackingEventType = "open"
	TrackingEventClick       TrackingEventType = "click"
	TrackingEventUnsubscribe TrackingEventType = "unsubscribe"
)

// TrackedLink stores rewritten links for click tracking.
type TrackedLink struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	CampaignID  uint      `json:"campaign_id" gorm:"index;not null"`
	OriginalURL string    `json:"original_url" gorm:"type:text;not null"`
	Hash        string    `json:"hash" gorm:"uniqueIndex;not null"`
	ClickCount  int64     `json:"click_count" gorm:"default:0;not null"`
	CreatedAt   time.Time `json:"created_at"`

	Campaign Campaign `json:"-" gorm:"foreignKey:CampaignID"`
}

// TrackingEvent records individual open/click/unsubscribe events.
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
