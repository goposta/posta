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

package upgrade

import "gorm.io/gorm"

// Step is a one-shot data migration that runs at boot time, exactly once per
// database. Schema migrations stay in GORM AutoMigrate; this registry is for
// data backfills, defaults, and cleanup that AutoMigrate cannot express.
//
// Rules each step author must follow:
//
//  1. ID is permanent. Once a step ships, never rename or delete its ID —
//     that's how the registry knows the step has already been applied.
//     Use a date-prefixed slug, e.g. "2026-05-30-mark-default-plan".
//  2. Apply must be idempotent in spirit even though the framework guarantees
//     single-run, because a step that crashes mid-way will retry on the next
//     boot. Use INSERT … ON CONFLICT DO NOTHING, WHERE NOT EXISTS, etc.
//  3. Apply runs inside its own transaction; do not start one yourself.
//  4. If a step is wrong after it ships, write a follow-up step that fixes
//     forward. Do not retroactively edit a step's Apply.
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
}

// Registry returns a copy of the registered steps so callers (status command,
// tests) can inspect the list without mutating it.
func Registry() []Step {
	out := make([]Step, len(registry))
	copy(out, registry)
	return out
}
