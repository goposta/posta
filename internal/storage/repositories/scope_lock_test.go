// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package repositories

import "testing"

func TestOwnsResourceIgnoresLegacyUserOwnership(t *testing.T) {
	ws := uint(7)
	other := uint(8)

	if !OwnsResource(ResourceScope{UserID: 1, WorkspaceID: &ws}, 999, &ws) {
		t.Fatal("a resource in the caller's workspace must be owned regardless of its user_id")
	}
	if OwnsResource(ResourceScope{UserID: 1, WorkspaceID: &ws}, 1, &other) {
		t.Fatal("a resource in another workspace must never be owned, even when user_id matches")
	}
	if OwnsResource(ResourceScope{UserID: 1, WorkspaceID: &ws}, 1, nil) {
		t.Fatal("an unscoped resource must not be owned once scoping is enforced")
	}
	if OwnsResource(ResourceScope{UserID: 1}, 1, nil) {
		t.Fatal("a scope with no workspace must own nothing")
	}
}
