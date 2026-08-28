// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package upgrade

import "testing"

func TestRegistryIDsAreUniqueAndOrdered(t *testing.T) {
	steps := Registry()
	seen := make(map[string]bool, len(steps))
	prev := ""

	for i, s := range steps {
		if s.ID == "" {
			t.Fatalf("step %d has an empty id", i)
		}
		if s.Apply == nil {
			t.Fatalf("step %q has no Apply func", s.ID)
		}
		if seen[s.ID] {
			t.Fatalf("step %q is registered twice; applied steps are keyed by id", s.ID)
		}
		seen[s.ID] = true

		// Only the date prefix must not go backwards. Steps sharing a date run in
		// dependency order, which is rarely alphabetical: 2026-08-28-system-workspace
		// has to precede the steps that read workspaces.system.
		date := s.ID
		if len(date) > 10 {
			date = date[:10]
		}
		if prev != "" && date < prev {
			t.Fatalf("step %q is dated before %q; the registry is append-only", s.ID, prev)
		}
		prev = date
	}
}

func TestRegistryIsACopy(t *testing.T) {
	a := Registry()
	if len(a) == 0 {
		t.Fatal("registry is empty")
	}
	a[0].ID = "mutated"

	if Registry()[0].ID == "mutated" {
		t.Fatal("Registry must hand out a copy; a caller mutated the in-tree registry")
	}
}

func TestWorkspaceTypeStepsAreRegistered(t *testing.T) {
	want := []string{
		"2026-08-28-system-workspace",
		"2026-08-28-personal-to-normal",
		"2026-08-28-default-workspace",
	}
	got := make(map[string]bool)
	for _, s := range Registry() {
		got[s.ID] = true
	}
	for _, id := range want {
		if !got[id] {
			t.Fatalf("upgrade step %q is not registered", id)
		}
	}
}
