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

type TemplateLocalization struct {
	ID              uint       `json:"id" gorm:"primaryKey"`
	VersionID       uint       `json:"version_id" gorm:"uniqueIndex:idx_version_language;not null"`
	Language        string     `json:"language" gorm:"uniqueIndex:idx_version_language;not null;size:10"`
	SubjectTemplate string     `json:"subject_template" gorm:"not null"`
	HTMLTemplate    string     `json:"html_template"`
	TextTemplate    string     `json:"text_template"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       *time.Time `json:"updated_at,omitempty"`

	Version TemplateVersion `json:"-" gorm:"foreignKey:VersionID;constraint:OnDelete:CASCADE"`
}
