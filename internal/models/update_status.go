// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

// UpdateStatus caches the result of the daily release check. Exactly one row
// (ID 1) ever exists: it is platform state, not a user-editable setting, so it
// deliberately lives outside the `settings` table — every row there is listed
// and editable on the admin Settings page.
type UpdateStatus struct {
	ID uint `json:"-" gorm:"primaryKey"`
	// LatestVersion is the newest release for the running build's channel, as a
	// semver tag with the leading "v" (e.g. "v0.13.0"). Empty until the first
	// successful check.
	LatestVersion  string     `json:"latest_version"`
	ReleaseURL     string     `json:"release_url"`
	PublishedAt    *time.Time `json:"published_at"`
	ETag           string     `json:"-"`
	CheckedVersion string     `json:"-"`
	CheckedAt      *time.Time `json:"checked_at"`
	LastError      string     `json:"last_error,omitempty"`
	// DismissedVersion is the version an admin chose to stop being notified about.
	// Platform-wide: only platform admins ever see the notice.
	DismissedVersion string    `json:"dismissed_version,omitempty"`
	UpdatedAt        time.Time `json:"-"`
}
