// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

type Template struct {
	ID              uint       `json:"id" gorm:"primaryKey"`
	UserID          uint       `json:"user_id" gorm:"index;not null"`
	WorkspaceID     *uint      `json:"workspace_id,omitempty" gorm:"index"`
	Name            string     `json:"name" gorm:"not null"`
	DefaultLanguage string     `json:"default_language" gorm:"default:en;not null"`
	ActiveVersionID *uint      `json:"active_version_id,omitempty" gorm:"uniqueIndex"`
	Description     string     `json:"description"`
	SampleData      string     `json:"sample_data"`
	LastEditedByID  *uint      `json:"last_edited_by_id,omitempty" gorm:"index"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       *time.Time `json:"updated_at,omitempty"`

	User          User             `json:"-" gorm:"foreignKey:UserID"`
	ActiveVersion *TemplateVersion `json:"active_version,omitempty" gorm:"foreignKey:ActiveVersionID;constraint:false"`
	CreatedBy     *ActorRef        `json:"created_by,omitempty" gorm:"foreignKey:UserID;references:ID;constraint:false"`
	LastEditedBy  *ActorRef        `json:"last_edited_by,omitempty" gorm:"foreignKey:LastEditedByID;references:ID;constraint:false"`
}
