// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package email

import "testing"

func TestResolveSenderWithDefault(t *testing.T) {
	cases := []struct {
		name        string
		from        string
		defaultName string
		want        string
	}{
		{"bare address gets default name", "hello@example.com", "Acme", "\"Acme\" <hello@example.com>"},
		{"existing display name is kept", "Support <hello@example.com>", "Acme", "Support <hello@example.com>"},
		{"empty default leaves bare address", "hello@example.com", "", "hello@example.com"},
		{"whitespace default leaves bare address", "hello@example.com", "   ", "hello@example.com"},
		{"unparseable input is untouched", "not-an-email", "Acme", "not-an-email"},
		{"name needing quoting is escaped", "hello@example.com", "Acme, Inc.", "\"Acme, Inc.\" <hello@example.com>"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveSenderWithDefault(tc.from, tc.defaultName); got != tc.want {
				t.Errorf("resolveSenderWithDefault(%q, %q) = %q, want %q", tc.from, tc.defaultName, got, tc.want)
			}
		})
	}
}
