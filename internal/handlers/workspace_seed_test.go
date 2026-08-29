// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package handlers

import "testing"

// Absent means seed. A client that predates the flag, or one that does not set
// it, should get a workspace it can send from rather than an empty one.
func TestShouldSeedWorkspace(t *testing.T) {
	yes, no := true, false

	cases := []struct {
		name string
		flag *bool
		want bool
	}{
		{"omitted", nil, true},
		{"explicitly true", &yes, true},
		{"explicitly false", &no, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldSeedWorkspace(tc.flag); got != tc.want {
				t.Fatalf("shouldSeedWorkspace(%v) = %v, want %v", tc.flag, got, tc.want)
			}
		})
	}
}
