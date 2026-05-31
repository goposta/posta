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

import "testing"

func TestCompare(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want int
	}{
		{"equal canonical", "v1.2.3", "v1.2.3", 0},
		{"missing v prefix", "1.2.3", "v1.2.3", 0},
		{"older patch", "v1.2.3", "v1.2.4", -1},
		{"older minor", "v1.2.0", "v1.3.0", -1},
		{"older major", "v1.9.9", "v2.0.0", -1},
		{"newer patch", "v1.2.4", "v1.2.3", 1},
		// Lexicographic vs numeric: would compare wrong as plain strings.
		{"numeric vs lexicographic minor", "v0.10.0", "v0.2.0", 1},
		{"numeric vs lexicographic patch", "v1.0.10", "v1.0.2", 1},
		// Pre-releases sort below the corresponding release.
		{"rc below release", "v1.0.0-rc.1", "v1.0.0", -1},
		{"rc1 before rc2", "v1.0.0-rc.1", "v1.0.0-rc.2", -1},
		// git describe suffixes — "v0.6.6-3-gabc1234" is post-tag commits.
		{"post-tag suffix sorts below next release", "v0.6.6-3-gabc1234", "v0.6.7", -1},
		{"post-tag suffix above older release", "v0.6.6-3-gabc1234", "v0.6.5", 1},
		{"dirty suffix still comparable", "v0.6.6-3-gabc1234-dirty", "v0.6.5", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Compare(tc.a, tc.b)
			if got != tc.want {
				t.Fatalf("Compare(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestIsDowngrade(t *testing.T) {
	cases := []struct {
		name           string
		binary, stored string
		want           bool
	}{
		{"newer binary not a downgrade", "v1.3.0", "v1.2.0", false},
		{"same version not a downgrade", "v1.3.0", "v1.3.0", false},
		{"older binary is a downgrade", "v1.2.0", "v1.3.0", true},
		{"dev binary never a downgrade", "dev", "v1.3.0", false},
		{"dev stored never a downgrade", "v1.3.0", "dev", false},
		{"empty binary treated as dev", "", "v1.0.0", false},

		{"post-tag ahead is not a downgrade", "v1.2.0-3-gabc1234", "v1.2.0", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsDowngrade(tc.binary, tc.stored); got != tc.want {
				t.Fatalf("IsDowngrade(%q, %q) = %v, want %v", tc.binary, tc.stored, got, tc.want)
			}
		})
	}
}

func TestIsDev(t *testing.T) {
	for _, v := range []string{"dev", ""} {
		if !IsDev(v) {
			t.Errorf("IsDev(%q) = false, want true", v)
		}
	}
	for _, v := range []string{"v1.0.0", "v0.6.6-3-gabc1234", "1.2.3"} {
		if IsDev(v) {
			t.Errorf("IsDev(%q) = true, want false", v)
		}
	}
}
