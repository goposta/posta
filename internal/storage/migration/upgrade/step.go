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

func Registry() []Step {
	out := make([]Step, len(registry))
	copy(out, registry)
	return out
}
