// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

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
