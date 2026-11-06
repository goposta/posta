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
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	MaxTags   = 20
	MaxTagLen = 32
)

// Commas are rejected because they would split the tag in a CSV export.
func NormalizeTag(raw string) (string, error) {
	tag := strings.ToLower(strings.Join(strings.Fields(raw), " "))
	if tag == "" {
		return "", fmt.Errorf("a tag cannot be empty")
	}
	if utf8.RuneCountInString(tag) > MaxTagLen {
		return "", fmt.Errorf("tag %q is longer than %d characters", tag, MaxTagLen)
	}
	for _, r := range tag {
		if unicode.IsControl(r) || r == ',' || r == ';' {
			return "", fmt.Errorf("tag %q contains a character tags cannot use", tag)
		}
	}
	return tag, nil
}

// Existing tags keep their place, and removing wins over adding the same tag.
func ApplyTags(current, add, remove []string) ([]string, error) {
	drop := map[string]bool{}
	for _, raw := range remove {
		tag, err := NormalizeTag(raw)
		if err != nil {
			return nil, err
		}
		drop[tag] = true
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(current)+len(add))
	keep := func(tag string) {
		if !drop[tag] && !seen[tag] {
			seen[tag] = true
			out = append(out, tag)
		}
	}
	for _, tag := range current {
		keep(tag)
	}
	for _, raw := range add {
		tag, err := NormalizeTag(raw)
		if err != nil {
			return nil, err
		}
		keep(tag)
	}
	if len(out) > MaxTags {
		return nil, fmt.Errorf("a message can carry at most %d tags", MaxTags)
	}
	return out, nil
}
