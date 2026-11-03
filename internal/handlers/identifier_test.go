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

package handlers

import "testing"

func TestResolveNames(t *testing.T) {
	cases := []struct {
		label                   string
		name, displayName, slug string
		wantName, wantDisplay   string
		wantErr                 bool
	}{
		{"both", "contact", "Contact form", "", "contact", "Contact form", false},
		{"display only", "", "Contact form", "", "contact-form", "Contact form", false},
		{"name only", "contact", "", "", "contact", "contact", false},
		{"legacy label in name", "Contact form", "", "", "contact-form", "Contact form", false},
		{"legacy name and slug", "Contact form", "", "contact", "contact", "Contact form", false},
		{"slug ignored with display_name", "contact", "Contact form", "other", "contact", "Contact form", false},
		{"bad name with display_name", "Contact form", "Contact", "", "", "", true},
		{"nothing", "", "", "", "", "", true},
		{"label without letters", "", "!!!", "", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			name, display, err := resolveNames(tc.name, tc.displayName, tc.slug)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, want error %v", err, tc.wantErr)
			}
			if !tc.wantErr && (name != tc.wantName || display != tc.wantDisplay) {
				t.Fatalf("got (%q, %q), want (%q, %q)", name, display, tc.wantName, tc.wantDisplay)
			}
		})
	}
}

func TestResolveRenames(t *testing.T) {
	// An old client renaming the label sent it as name.
	if name, display, err := resolveRenames("New label", "", ""); err != nil || name != "" || display != "New label" {
		t.Fatalf("legacy label rename = (%q, %q, %v)", name, display, err)
	}
	if name, display, err := resolveRenames("new-id", "", ""); err != nil || name != "new-id" || display != "" {
		t.Fatalf("identifier rename = (%q, %q, %v)", name, display, err)
	}
	if _, _, err := resolveRenames("Bad Id", "Label", ""); err == nil {
		t.Fatal("a malformed name with a display name must be refused")
	}
}

func TestImportNames(t *testing.T) {
	if n, d := importNames("Welcome Email", "", "template"); n != "welcome-email" || d != "Welcome Email" {
		t.Fatalf("old export = (%q, %q)", n, d)
	}
	if n, d := importNames("welcome", "Welcome", "template"); n != "welcome" || d != "Welcome" {
		t.Fatalf("new export = (%q, %q)", n, d)
	}
	if n, d := importNames("", "!!!", "template"); n != "template" || d != "!!!" {
		t.Fatalf("no letters = (%q, %q)", n, d)
	}
}
