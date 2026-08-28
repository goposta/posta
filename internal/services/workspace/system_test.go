// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package workspace

import "testing"

func TestIsReservedSlug(t *testing.T) {
	if !IsReservedSlug(SystemSlug) {
		t.Fatalf("IsReservedSlug(%q) = false, want true", SystemSlug)
	}
	for _, slug := range []string{"acme", "system-2", "systems", "", "System"} {
		if IsReservedSlug(slug) {
			t.Fatalf("IsReservedSlug(%q) = true, want false", slug)
		}
	}
}
