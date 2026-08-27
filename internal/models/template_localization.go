// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

type TemplateLocalization struct {
	ID              uint       `json:"id" gorm:"primaryKey"`
	VersionID       uint       `json:"version_id" gorm:"uniqueIndex:idx_version_language;not null"`
	Language        string     `json:"language" gorm:"uniqueIndex:idx_version_language;not null;size:10"`
	SubjectTemplate string     `json:"subject_template" gorm:"not null"`
	HTMLTemplate    string     `json:"html_template"`
	TextTemplate    string     `json:"text_template"`
	BuilderJSON     string     `json:"builder_json,omitempty" gorm:"type:text"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       *time.Time `json:"updated_at,omitempty"`

	Version TemplateVersion `json:"-" gorm:"foreignKey:VersionID;constraint:OnDelete:CASCADE"`
}
