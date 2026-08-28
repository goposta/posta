// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

// Setting value types. The frontend renders a control from this hint, so the
// vocabulary is closed: a typo produces a text box where a checkbox belongs.
const (
	SettingTypeString = "string"
	SettingTypeBool   = "bool"
	SettingTypeInt    = "int"
)

// Serialized values for a SettingTypeBool entry. Values are stored as text, so
// these are the only two a bool setting may hold.
const (
	SettingTrue  = "true"
	SettingFalse = "false"
)

// Setting represents a platform-wide configuration entry managed by admins.
// Settings are stored as key-value pairs with a type hint for the frontend.
type Setting struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Key       string    `json:"key" gorm:"uniqueIndex;not null"`
	Value     string    `json:"value" gorm:"type:text"`
	Type      string    `json:"type" gorm:"default:string;not null"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
