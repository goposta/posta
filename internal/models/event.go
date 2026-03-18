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

type EventCategory string

const (
	EventCategoryUser   EventCategory = "user"
	EventCategoryEmail  EventCategory = "email"
	EventCategorySystem EventCategory = "system"
	EventCategoryAudit  EventCategory = "audit"
)

type Event struct {
	ID        uint          `json:"id" gorm:"primaryKey"`
	Category  EventCategory `json:"category" gorm:"index;not null"`
	Type      string        `json:"type" gorm:"index;not null"`
	ActorID   *uint         `json:"actor_id" gorm:"index"`
	ActorName string        `json:"actor_name"`
	ClientIP  string        `json:"client_ip,omitempty" gorm:"size:45"`
	Message   string        `json:"message" gorm:"not null"`
	Metadata  string        `json:"metadata" gorm:"type:text"`
	CreatedAt time.Time     `json:"created_at" gorm:"index"`
}
