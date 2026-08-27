// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

type UpgradeStep struct {
	ID         string    `json:"id" gorm:"primaryKey;size:128"`
	AppVersion string    `json:"app_version" gorm:"size:64;not null"`
	AppliedAt  time.Time `json:"applied_at" gorm:"not null"`
	DurationMS int64     `json:"duration_ms"`
}
