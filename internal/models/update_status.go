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
