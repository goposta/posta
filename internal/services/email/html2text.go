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

package email

import (
	"html"
	"regexp"
	"strings"
)

// HTMLToText converts an HTML string to readable plain text.
// It strips tags, converts common block elements to appropriate whitespace,
// extracts link URLs, and decodes HTML entities.
func HTMLToText(input string) string {
	if input == "" {
		return ""
	}

	s := input

	// Convert <br>, <br/>, <br /> to newlines
	brRe := regexp.MustCompile(`(?i)<br\s*/?>`)
	s = brRe.ReplaceAllString(s, "\n")

	// Convert </p> to double newlines
	closePRe := regexp.MustCompile(`(?i)</p\s*>`)
	s = closePRe.ReplaceAllString(s, "\n\n")

	// Convert <p ...> to double newlines (except the very first one)
	openPRe := regexp.MustCompile(`(?i)<p[^>]*>`)
	s = openPRe.ReplaceAllString(s, "\n\n")

	// Convert <li> to "- " prefix
	liRe := regexp.MustCompile(`(?i)<li[^>]*>`)
	s = liRe.ReplaceAllString(s, "\n- ")

	// Convert <a href="url">text</a> to "text (url)"
	linkRe := regexp.MustCompile(`(?i)<a\s[^>]*href\s*=\s*["']([^"']*)["'][^>]*>(.*?)</a>`)
	s = linkRe.ReplaceAllString(s, "$2 ($1)")

	// Strip all remaining HTML tags
	tagRe := regexp.MustCompile(`<[^>]*>`)
	s = tagRe.ReplaceAllString(s, "")

	// Decode HTML entities (handles &amp;, &lt;, &gt;, &nbsp;, &#xxxx; etc.)
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = html.UnescapeString(s)

	// Collapse runs of spaces/tabs on each line (preserve newlines)
	spaceRe := regexp.MustCompile(`[^\S\n]+`)
	s = spaceRe.ReplaceAllString(s, " ")

	// Collapse 3+ consecutive newlines into 2
	nlRe := regexp.MustCompile(`\n{3,}`)
	s = nlRe.ReplaceAllString(s, "\n\n")

	s = strings.TrimSpace(s)

	return s
}
