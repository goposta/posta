// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

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
