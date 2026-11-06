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

import (
	"regexp"
	"strings"
)

var styleBlockRegex = regexp.MustCompile(`(?is)<style[^>]*>(.*?)</style>`)

// extractStyleAndBody also strips stylesheet <link> tags and returns only the <body> content when present.
func extractStyleAndBody(htmlContent string) (css string, body string) {
	var cssBlocks []string
	matches := styleBlockRegex.FindAllStringSubmatch(htmlContent, -1)
	for _, m := range matches {
		if len(m) >= 2 {
			trimmed := strings.TrimSpace(m[1])
			if trimmed != "" {
				cssBlocks = append(cssBlocks, trimmed)
			}
		}
	}
	css = strings.Join(cssBlocks, "\n\n")

	cleaned := styleBlockRegex.ReplaceAllString(htmlContent, "")
	cleaned = stripStyleSheetLinks(cleaned)

	body = extractBody(cleaned)

	return css, body
}

var (
	linkTagRegex  = regexp.MustCompile(`(?is)<link\b[^>]*>`)
	relAttrRegex  = regexp.MustCompile(`(?is)\brel\s*=\s*["']?([^"'>\s]+)`)
	hrefAttrRegex = regexp.MustCompile(`(?is)\bhref\s*=\s*["']([^"']+)["']`)
)

func isStyleSheetLink(tag string) bool {
	rel := relAttrRegex.FindStringSubmatch(tag)
	return len(rel) >= 2 && strings.Contains(strings.ToLower(rel[1]), "stylesheet")
}

// Names are each href's basename without .css, e.g. "css/styles.css?v=2" -> "styles".
func extractStyleSheetLinks(htmlContent string) []string {
	var names []string
	for _, tag := range linkTagRegex.FindAllString(htmlContent, -1) {
		if !isStyleSheetLink(tag) {
			continue
		}
		href := hrefAttrRegex.FindStringSubmatch(tag)
		if len(href) < 2 {
			continue
		}
		if name := styleSheetNameFromHref(href[1]); name != "" {
			names = append(names, name)
		}
	}
	return names
}

func stripStyleSheetLinks(htmlContent string) string {
	return linkTagRegex.ReplaceAllStringFunc(htmlContent, func(tag string) string {
		if isStyleSheetLink(tag) {
			return ""
		}
		return tag
	})
}

func styleSheetNameFromHref(href string) string {
	h := strings.TrimSpace(href)
	if i := strings.IndexAny(h, "?#"); i >= 0 {
		h = h[:i]
	}
	h = strings.TrimRight(h, "/")
	if i := strings.LastIndexAny(h, `/\`); i >= 0 {
		h = h[i+1:]
	}
	h = strings.TrimSuffix(h, ".css")
	return strings.TrimSpace(h)
}

var bodyRegex = regexp.MustCompile(`(?is)<body[^>]*>(.*)</body>`)

// extractBody returns the trimmed full HTML when there is no <body> tag.
func extractBody(html string) string {
	m := bodyRegex.FindStringSubmatch(html)
	if len(m) >= 2 {
		return strings.TrimSpace(m[1])
	}
	return strings.TrimSpace(html)
}

func extractTitle(html string) string {
	re := regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	m := re.FindStringSubmatch(html)
	if len(m) >= 2 {
		return strings.TrimSpace(m[1])
	}
	return ""
}
