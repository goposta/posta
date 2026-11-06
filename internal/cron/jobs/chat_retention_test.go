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

package jobs

import "testing"

func TestTruncateIP(t *testing.T) {
	cases := map[string]string{
		"203.0.113.77":          "203.0.113.0",
		"203.0.113.0":           "203.0.113.0",
		"2001:db8:1234:5678::1": "2001:db8:1234::",
		"::ffff:198.51.100.9":   "198.51.100.0",
		"not an ip":             "",
		"":                      "",
	}
	for in, want := range cases {
		if got := TruncateIP(in); got != want {
			t.Errorf("TruncateIP(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEffectiveChatRetention(t *testing.T) {
	cases := []struct{ platform, widget, want int }{
		{365, 0, 365},
		{365, 30, 30},
		{365, 400, 365}, // a widget can only lower it
		{0, 90, 90},     // platform keeps forever, widget chose 90
		{0, 0, 0},
	}
	for _, c := range cases {
		if got := EffectiveChatRetention(c.platform, c.widget); got != c.want {
			t.Errorf("EffectiveChatRetention(%d, %d) = %d, want %d", c.platform, c.widget, got, c.want)
		}
	}
}
