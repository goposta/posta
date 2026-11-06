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

package email

import (
	"html"
	"regexp"
	"strings"
)

func HTMLToText(input string) string {
	if input == "" {
		return ""
	}

	s := input

	brRe := regexp.MustCompile(`(?i)<br\s*/?>`)
	s = brRe.ReplaceAllString(s, "\n")

	closePRe := regexp.MustCompile(`(?i)</p\s*>`)
	s = closePRe.ReplaceAllString(s, "\n\n")

	openPRe := regexp.MustCompile(`(?i)<p[^>]*>`)
	s = openPRe.ReplaceAllString(s, "\n\n")

	liRe := regexp.MustCompile(`(?i)<li[^>]*>`)
	s = liRe.ReplaceAllString(s, "\n- ")

	linkRe := regexp.MustCompile(`(?i)<a\s[^>]*href\s*=\s*["']([^"']*)["'][^>]*>(.*?)</a>`)
	s = linkRe.ReplaceAllString(s, "$2 ($1)")

	tagRe := regexp.MustCompile(`<[^>]*>`)
	s = tagRe.ReplaceAllString(s, "")

	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = html.UnescapeString(s)

	spaceRe := regexp.MustCompile(`[^\S\n]+`)
	s = spaceRe.ReplaceAllString(s, " ")

	nlRe := regexp.MustCompile(`\n{3,}`)
	s = nlRe.ReplaceAllString(s, "\n\n")

	s = strings.TrimSpace(s)

	return s
}
