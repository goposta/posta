/*
 * Copyright 2026 Jonas Kaninda
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package handlers

import (
	"regexp"
	"strings"
)

var styleBlockRegex = regexp.MustCompile(`(?is)<style[^>]*>(.*?)</style>`)

// extractStyleAndBody extracts CSS from <style> blocks and returns the cleaned HTML body.
// If the HTML contains a <body> tag, only the body content is returned.
// All <style> blocks are removed from the returned HTML.
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

	// Remove <style> blocks from HTML
	cleaned := styleBlockRegex.ReplaceAllString(htmlContent, "")

	// Extract <body> content if present
	body = extractBody(cleaned)

	return css, body
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
