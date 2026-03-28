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

type Contact struct {
	ID          uint       `json:"id" gorm:"primaryKey"`
	UserID      uint       `json:"user_id" gorm:"index;not null"`
	WorkspaceID *uint      `json:"workspace_id,omitempty" gorm:"index"`
	Email      string     `json:"email" gorm:"not null"`
	Name       string     `json:"name" gorm:"default:''"`
	SentCount  int64      `json:"sent_count" gorm:"default:0;not null"`
	FailCount  int64      `json:"fail_count" gorm:"default:0;not null"`
	LastSentAt *time.Time `json:"last_sent_at"`
	CreatedAt  time.Time  `json:"created_at"`
}
