// Copyright (c) 2026 Jonas Kaninda.
//
// This file is part of Posta.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package handlers

import "testing"

// Absent means seed, so older clients get a workspace they can send from rather than an empty one.
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
