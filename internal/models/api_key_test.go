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
