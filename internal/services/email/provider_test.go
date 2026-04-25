/*
 * Copyright 2026 Jonas Kaninda
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 */

package email

import "testing"

func TestNormalizeDomain(t *testing.T) {
	cases := map[string]string{
		"a@gmail.com":                 "gmail.com",
		"A@Gmail.COM":                 "gmail.com",
		`"Jonas" <j@gmail.com>`:       "gmail.com",
		`Jonas Kaninda <j@GMAIL.com>`: "gmail.com",
		"  x@OUTLOOK.com  ":           "outlook.com",
		"no-at-sign":                  "",
		"":                            "",
		"weird<j@gmail.com>trail":     "gmail.com",
	}
	for in, want := range cases {
		if got := NormalizeDomain(in); got != want {
			t.Errorf("NormalizeDomain(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestClassifyProvider(t *testing.T) {
	cases := map[string]string{
		"a@gmail.com":       "Gmail",
		"a@googlemail.com":  "Gmail",
		"ceo@google.com":    "Google Workspace",
		"x@mail.google.com": "Google Workspace",
		"x@corp.google.com": "Google Workspace",
		"x@outlook.com":     "Outlook",
		"x@hotmail.co.uk":   "Outlook",
		"x@yahoo.fr":        "Yahoo",
		"x@icloud.com":      "Apple iCloud",
		"x@proton.me":       "Proton",
		"x@pm.me":           "Proton",
		"x@aol.com":         "AOL",
		"x@gmx.de":          "GMX",
		"x@zoho.com":        "Zoho",
		"x@fastmail.fm":     "Fastmail",
		"x@yandex.ru":       "Yandex",
		"x@example.com":     "Other",
		"":                  "Other",
		"bad-address":       "Other",
		`"J" <j@Gmail.com>`: "Gmail",
	}
	for in, want := range cases {
		if got := ClassifyProvider(in); got != want {
			t.Errorf("ClassifyProvider(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestClassifyRecipients(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{[]string{"a@gmail.com"}, "Gmail"},
		{[]string{"a@example.com", "b@gmail.com"}, "Other"}, // first parseable wins
		{[]string{"", "a@yahoo.com"}, "Yahoo"},
		{nil, "Other"},
		{[]string{}, "Other"},
		{[]string{""}, "Other"},
	}
	for _, tc := range cases {
		if got := ClassifyRecipients(tc.in); got != tc.want {
			t.Errorf("ClassifyRecipients(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// Defense against the original bug: RFC 5322 display names must not leak into
// the domain column. With the old split_part SQL, this produced "gmail.com>".
func TestRFC5322DisplayNameIsStripped(t *testing.T) {
	got := NormalizeDomain(`"Jonas Kaninda" <jonas@gmail.com>`)
	if got != "gmail.com" {
		t.Fatalf("display-name stripping failed: got %q, want %q", got, "gmail.com")
	}
}
