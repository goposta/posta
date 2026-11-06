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

package chat

import (
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Bodies are plain text and never rendered as HTML, so only control characters are stripped, nothing escaped.
func CleanBody(s string) string {
	s = norm.NFC.String(s)
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if r == '\r' || unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	return strings.TrimSpace(s)
}

func CleanLine(s string, max int) string {
	s = strings.Join(strings.Fields(CleanBody(s)), " ")
	if r := []rune(s); len(r) > max {
		s = string(r[:max])
	}
	return s
}

var accentPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// The colour ends up in a stylesheet on the customer's page, so nothing but #RRGGBB may pass.
func ValidAccentColor(c string) bool {
	return c == "" || accentPattern.MatchString(c)
}

func ValidAvatarURL(u string) bool {
	if u == "" {
		return true
	}
	parsed, err := url.Parse(u)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && len(u) <= 2000
}

// CleanPageURL keeps only short http(s) URLs, since agents see the page as a link.
func CleanPageURL(u string) string {
	u = strings.TrimSpace(u)
	if u == "" || len(u) > 2000 {
		return ""
	}
	parsed, err := url.Parse(u)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return ""
	}
	return u
}
