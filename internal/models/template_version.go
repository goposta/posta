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

type TemplateVersion struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	TemplateID   uint      `json:"template_id" gorm:"index;not null"`
	Version      int       `json:"version" gorm:"not null"`
	StyleSheetID *uint     `json:"stylesheet_id,omitempty" gorm:"index"`
	SampleData   string    `json:"sample_data"`
	CreatedAt    time.Time `json:"created_at"`

	StyleSheet    *StyleSheet            `json:"stylesheet,omitempty" gorm:"foreignKey:StyleSheetID;constraint:OnDelete:SET NULL"`
	Localizations []TemplateLocalization `json:"localizations,omitempty" gorm:"foreignKey:VersionID"`
}
