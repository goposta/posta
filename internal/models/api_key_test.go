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

package models

import (
	"testing"

	"github.com/lib/pq"
)

func TestAPIKeyHasScope(t *testing.T) {
	cases := []struct {
		name   string
		scopes []string
		query  string
		want   bool
	}{
		{"empty defaults to send", nil, ScopeSend, true},
		{"empty denies read", nil, ScopeRead, false},
		{"empty denies webhooks", nil, ScopeWebhooks, false},
		{"explicit read grants read", []string{ScopeRead}, ScopeRead, true},
		{"explicit read denies send", []string{ScopeRead}, ScopeSend, false},
		{"wildcard grants read", []string{ScopeAll}, ScopeRead, true},
		{"wildcard grants webhooks", []string{ScopeAll}, ScopeWebhooks, true},
		{"multi grants both", []string{ScopeSend, ScopeWebhooks}, ScopeWebhooks, true},
		{"multi denies missing", []string{ScopeSend, ScopeWebhooks}, ScopeRead, false},

		// write and admin are workspace-level scopes covered by the wildcard; neither confers platform administration.
		{"wildcard grants write", []string{ScopeAll}, ScopeWrite, true},
		{"wildcard grants admin", []string{ScopeAll}, ScopeAdmin, true},
		{"explicit write grants write", []string{ScopeWrite}, ScopeWrite, true},
		{"explicit admin grants admin", []string{ScopeAdmin}, ScopeAdmin, true},
		{"admin alone denies send", []string{ScopeAdmin}, ScopeSend, false},
		{"empty denies write", nil, ScopeWrite, false},
		{"empty denies admin", nil, ScopeAdmin, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k := &APIKey{Scopes: pq.StringArray(tc.scopes)}
			if got := k.HasScope(tc.query); got != tc.want {
				t.Errorf("HasScope(%q) with scopes %v = %v, want %v", tc.query, tc.scopes, got, tc.want)
			}
		})
	}
}
