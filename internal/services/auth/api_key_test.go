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

package auth

import (
	"reflect"
	"testing"

	"github.com/goposta/posta/internal/models"
)

func TestNormalizeScopes(t *testing.T) {
	cases := []struct {
		name    string
		in      []string
		want    []string
		wantErr bool
	}{
		{"nil defaults to send", nil, []string{models.ScopeSend}, false},
		{"empty defaults to send", []string{}, []string{models.ScopeSend}, false},
		{"passes through valid", []string{models.ScopeRead, models.ScopeWebhooks}, []string{models.ScopeRead, models.ScopeWebhooks}, false},
		{"dedupes", []string{models.ScopeSend, models.ScopeSend, models.ScopeRead}, []string{models.ScopeSend, models.ScopeRead}, false},
		{"wildcard is valid", []string{models.ScopeAll}, []string{models.ScopeAll}, false},
		{"unknown scope errors", []string{"delete-everything"}, nil, true},
		{"setup rejected until phase 2", []string{models.ScopeSetup}, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeScopes(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("NormalizeScopes(%v) = %v, want error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeScopes(%v) unexpected error: %v", tc.in, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("NormalizeScopes(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
