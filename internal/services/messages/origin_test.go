// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package messages

import (
	"testing"

	"github.com/goposta/posta/internal/models"
	"github.com/lib/pq"
)

func TestOriginAllowed(t *testing.T) {
	cases := []struct {
		name    string
		origins []string
		strict  bool
		origin  string
		want    bool
	}{
		{"no allowlist accepts anything", nil, false, "https://evil.test", true},
		{"empty origin allowed by default", []string{"https://example.com"}, false, "", true},
		{"empty origin rejected when strict", []string{"https://example.com"}, true, "", false},
		{"exact match", []string{"https://example.com"}, false, "https://example.com", true},
		{"trailing slash tolerated", []string{"https://example.com"}, false, "https://example.com/", true},
		{"scheme mismatch", []string{"https://example.com"}, false, "http://example.com", false},
		{"port mismatch", []string{"https://example.com"}, false, "https://example.com:8443", false},
		{"host mismatch", []string{"https://example.com"}, false, "https://evil.com", false},
		{"subdomain is not a match", []string{"https://example.com"}, false, "https://www.example.com", false},
		{"wildcard", []string{"*"}, false, "https://anything.test", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			form := &models.Form{AllowedOrigins: pq.StringArray(tc.origins), StrictOrigin: tc.strict}
			if got := OriginAllowed(form, tc.origin); got != tc.want {
				t.Fatalf("OriginAllowed(%q) = %v, want %v", tc.origin, got, tc.want)
			}
		})
	}
}

func TestRedirectAllowedRejectsOpenRedirects(t *testing.T) {
	form := &models.Form{AllowedOrigins: pq.StringArray{"https://example.com"}}

	cases := []struct {
		target string
		want   bool
	}{
		{"https://example.com/thanks", true},
		{"https://evil.test/phish", false},
		{"javascript:alert(1)", false},
		{"//evil.test", false},
		{"/thanks", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := RedirectAllowed(form, tc.target); got != tc.want {
			t.Fatalf("RedirectAllowed(%q) = %v, want %v", tc.target, got, tc.want)
		}
	}
}

func TestRedirectAllowedWithoutAllowlistRejects(t *testing.T) {
	form := &models.Form{}
	if RedirectAllowed(form, "https://anywhere.test/thanks") {
		t.Fatal("a form with no allowlist must not honour an arbitrary redirect target")
	}
}

func TestRedirectAllowedMatchesConfiguredRedirectHost(t *testing.T) {
	form := &models.Form{RedirectURL: "https://example.com/thanks"}
	if !RedirectAllowed(form, "https://example.com/other") {
		t.Fatal("a target on the configured redirect host should be allowed")
	}
}
