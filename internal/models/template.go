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

type Template struct {
	ID              uint       `json:"id" gorm:"primaryKey"`
	UserID          uint       `json:"user_id" gorm:"uniqueIndex:idx_user_template;not null"`
	Name            string     `json:"name" gorm:"uniqueIndex:idx_user_template;not null"`
	DefaultLanguage string     `json:"default_language" gorm:"default:en;not null"`
	ActiveVersionID *uint      `json:"active_version_id,omitempty" gorm:"uniqueIndex"`
	Description     string     `json:"description"`
	SampleData      string     `json:"sample_data"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       *time.Time `json:"updated_at,omitempty"`

	User          User             `json:"-" gorm:"foreignKey:UserID"`
	ActiveVersion *TemplateVersion `json:"active_version,omitempty" gorm:"foreignKey:ActiveVersionID;constraint:false"`
}
