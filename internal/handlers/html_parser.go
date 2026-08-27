// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package handlers

import (
	"regexp"
	"strings"
)

var styleBlockRegex = regexp.MustCompile(`(?is)<style[^>]*>(.*?)</style>`)

// extractStyleAndBody extracts CSS from <style> blocks and returns the cleaned HTML body.
// If the HTML contains a <body> tag, only the body content is returned.
// All <style> blocks and stylesheet <link> tags are removed from the returned HTML.
func extractStyleAndBody(htmlContent string) (css string, body string) {
	// Extract all <style> block contents
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

	// Remove <style> blocks and stylesheet <link> tags from HTML
	cleaned := styleBlockRegex.ReplaceAllString(htmlContent, "")
	cleaned = stripStyleSheetLinks(cleaned)

	// Extract <body> content if present
	body = extractBody(cleaned)

	return css, body
}

var (
	linkTagRegex  = regexp.MustCompile(`(?is)<link\b[^>]*>`)
	relAttrRegex  = regexp.MustCompile(`(?is)\brel\s*=\s*["']?([^"'>\s]+)`)
	hrefAttrRegex = regexp.MustCompile(`(?is)\bhref\s*=\s*["']([^"']+)["']`)
)

// isStyleSheetLink reports whether a <link ...> tag references a stylesheet.
func isStyleSheetLink(tag string) bool {
	rel := relAttrRegex.FindStringSubmatch(tag)
	return len(rel) >= 2 && strings.Contains(strings.ToLower(rel[1]), "stylesheet")
}

// extractStyleSheetLinks returns the stylesheet names referenced by
// <link rel="stylesheet" href="..."> tags, derived from each href's basename
// without its .css extension (e.g. "css/styles.css?v=2" -> "styles").
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

// extractBody extracts the inner content of the <body> tag.
// If no <body> tag is found, returns the full HTML (trimmed).
func extractBody(html string) string {
	m := bodyRegex.FindStringSubmatch(html)
	if len(m) >= 2 {
		return strings.TrimSpace(m[1])
	}
	return strings.TrimSpace(html)
}

// extractTitle extracts the content of the <title> tag, if present.
func extractTitle(html string) string {
	re := regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	m := re.FindStringSubmatch(html)
	if len(m) >= 2 {
		return strings.TrimSpace(m[1])
	}
	return ""
}
