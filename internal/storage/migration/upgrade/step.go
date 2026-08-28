// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package upgrade

import "gorm.io/gorm"

type Step struct {
	ID    string
	Apply func(tx *gorm.DB) error
}

// registry is the in-tree list of upgrade steps, in the order they should be
// applied. Append-only: new steps go at the end, old steps never move.
//
// To register a step, add an entry here and define its Apply func in a sibling
// file (e.g. steps_2026_05_30_default_plan.go).
var registry = []Step{
	{

		ID:    "2026-05-31-personal-workspaces",
		Apply: applyPersonalWorkspaces,
	},
	{
		ID:    "2026-06-04-dedupe-contacts",
		Apply: applyDedupeContacts,
	},
	{
		ID:    "2026-06-11-api-key-scopes",
		Apply: applyAPIKeyScopes,
	},
	{
		ID:    "2026-08-28-system-workspace",
		Apply: applySystemWorkspace,
	},
	{
		ID:    "2026-08-28-personal-to-normal",
		Apply: applyPersonalToNormal,
	},
	{
		ID:    "2026-08-28-default-workspace",
		Apply: applyDefaultWorkspace,
	},
}

func Registry() []Step {
	out := make([]Step, len(registry))
	copy(out, registry)
	return out
}
