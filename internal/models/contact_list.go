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

type ContactList struct {
	ID          uint       `json:"id" gorm:"primaryKey"`
	UserID      uint       `json:"user_id" gorm:"index;not null"`
	Name        string     `json:"name" gorm:"not null"`
	Description string     `json:"description"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at"`

	User    User                `json:"-" gorm:"foreignKey:UserID"`
	Members []ContactListMember `json:"-" gorm:"foreignKey:ListID"`
}

type ContactListMember struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	ListID    uint      `json:"list_id" gorm:"uniqueIndex:idx_list_email;not null"`
	Email     string    `json:"email" gorm:"uniqueIndex:idx_list_email;not null"`
	Name      string    `json:"name"`
	Data      string    `json:"data" gorm:"type:text"` // JSON metadata
	CreatedAt time.Time `json:"created_at"`

	List ContactList `json:"-" gorm:"foreignKey:ListID"`
}
