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

package messages

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeTag(t *testing.T) {
	cases := map[string]string{
		"VIP":            "vip",
		"  follow   up ": "follow up",
		"Ünïcode":        "ünïcode",
	}
	for in, want := range cases {
		got, err := NormalizeTag(in)
		if err != nil || got != want {
			t.Errorf("NormalizeTag(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "   ", "a,b", "a;b", "tab\x00", strings.Repeat("x", MaxTagLen+1)} {
		if _, err := NormalizeTag(bad); err == nil {
			t.Errorf("NormalizeTag(%q) accepted", bad)
		}
	}
}

func TestApplyTags(t *testing.T) {
	got, err := ApplyTags([]string{"vip", "lead"}, []string{"Hot", "VIP", "later"}, []string{"lead", "LATER"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"vip", "hot"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	if got, _ := ApplyTags(nil, nil, []string{"absent"}); len(got) != 0 {
		t.Errorf("removing from nothing gave %v", got)
	}
	if _, err := ApplyTags(nil, []string{"ok", ","}, nil); err == nil {
		t.Error("a bad tag in add was accepted")
	}

	many := make([]string, MaxTags+1)
	for i := range many {
		many[i] = fmt.Sprintf("t%d", i)
	}
	if _, err := ApplyTags(nil, many, nil); err == nil {
		t.Errorf("%d tags accepted", len(many))
	}
}
