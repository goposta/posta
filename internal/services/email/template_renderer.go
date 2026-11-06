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
	"bytes"
	"fmt"
	htmltmpl "html/template"
	"regexp"
	"strings"
	texttmpl "text/template"

	"github.com/jkaninda/okapi"
	"github.com/vanng822/go-premailer/premailer"
)

var actionRe = regexp.MustCompile(`\{\{(-?)\s*(.*?)\s*(-?)\}\}`)

var identRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// controlKeywords are Go template control keywords that must never be dot-prefixed.
var controlKeywords = map[string]struct{}{
	"range":    {},
	"end":      {},
	"if":       {},
	"else":     {},
	"with":     {},
	"template": {},
	"block":    {},
	"define":   {},
	"nil":      {},
	"true":     {},
	"false":    {},
	"and":      {},
	"or":       {},
	"not":      {},
	"call":     {},
	"html":     {},
	"js":       {},
	"urlquery": {},
	"print":    {},
	"println":  {},
	"printf":   {},
	"index":    {},
	"len":      {},
	"eq":       {},
	"ne":       {},
	"lt":       {},
	"le":       {},
	"gt":       {},
	"ge":       {},
}

// normalizeTemplate dot-prefixes bare identifiers in actions so they resolve against data, not as functions.
func normalizeTemplate(s string) string {
	return actionRe.ReplaceAllStringFunc(s, func(match string) string {
		m := actionRe.FindStringSubmatch(match)
		if m == nil {
			return match
		}
		leftDash := m[1]
		inner := m[2]
		rightDash := m[3]

		open := "{{" + leftDash
		close := rightDash + "}}"

		normalized := normalizeParts(inner)
		return open + " " + normalized + " " + close
	})
}

func normalizeParts(inner string) string {
	parts := strings.Fields(inner)
	if len(parts) == 0 {
		return inner
	}

	first := parts[0]

	if len(parts) == 1 {
		return dotPrefix(first)
	}

	if _, isKeyword := controlKeywords[first]; isKeyword {
		normalized := make([]string, len(parts))
		normalized[0] = first
		for i, p := range parts[1:] {
			normalized[i+1] = dotPrefix(p)
		}
		return strings.Join(normalized, " ")
	}

	return inner
}

func dotPrefix(token string) string {
	if token == "." ||
		strings.HasPrefix(token, ".") ||
		strings.HasPrefix(token, "$") {
		return token
	}
	if _, isKeyword := controlKeywords[token]; isKeyword {
		return token
	}
	if identRe.MatchString(token) {
		return "." + token
	}
	return token
}

const (
	missingKeyError   = "error"   // return an error on a missing data key (default)
	missingKeyZero    = "zero"    // silently use the zero value
	missingKeyInvalid = "invalid" // render "<no value>"
)

type TemplateRenderer struct {
	MissingKeyBehavior string
}

func NewTemplateRenderer() *TemplateRenderer {
	return &TemplateRenderer{
		MissingKeyBehavior: missingKeyError,
	}
}

type RenderInput struct {
	SubjectTemplate string
	HTMLTemplate    string
	TextTemplate    string
	CSS             string
}

type RenderedTemplate struct {
	Subject string
	HTML    string
	Text    string
}

func (r *TemplateRenderer) Render(input *RenderInput, data okapi.M) (*RenderedTemplate, error) {
	missingKey := r.missingKeyOption()

	subject, err := renderText("subject", normalizeTemplate(input.SubjectTemplate), data, missingKey)
	if err != nil {
		return nil, fmt.Errorf("failed to render subject: %w", err)
	}

	var html string
	if input.HTMLTemplate != "" {
		html, err = renderHTML("html", normalizeTemplate(input.HTMLTemplate), data, missingKey)
		if err != nil {
			return nil, fmt.Errorf("failed to render HTML: %w", err)
		}
		if input.CSS != "" {
			html = injectCSS(html, input.CSS)
		}
	}

	var text string
	if input.TextTemplate != "" {
		text, err = renderText("text", normalizeTemplate(input.TextTemplate), data, missingKey)
		if err != nil {
			return nil, fmt.Errorf("failed to render text: %w", err)
		}
	}

	return &RenderedTemplate{
		Subject: subject,
		HTML:    html,
		Text:    text,
	}, nil
}

func (r *TemplateRenderer) missingKeyOption() string {
	switch r.MissingKeyBehavior {
	case missingKeyZero, missingKeyInvalid:
		return r.MissingKeyBehavior
	default:
		return missingKeyError
	}
}

// injectCSS inlines styles with premailer for email client compatibility.
func injectCSS(html, css string) string {
	styleTag := "<style>\n" + css + "\n</style>"
	htmlLower := strings.ToLower(html)

	var withStyle string
	switch {
	case strings.Contains(htmlLower, "</head>"):
		idx := strings.Index(htmlLower, "</head>")
		withStyle = html[:idx] + styleTag + "\n" + html[idx:]
	case strings.Contains(htmlLower, "<body"):
		// Inject after opening <body> tag, not before </body>
		idx := strings.Index(htmlLower, "<body")
		end := strings.Index(html[idx:], ">")
		if end != -1 {
			insertAt := idx + end + 1
			withStyle = html[:insertAt] + "\n" + styleTag + html[insertAt:]
		} else {
			withStyle = styleTag + "\n" + html
		}
	case strings.Contains(htmlLower, "</body>"):
		idx := strings.Index(htmlLower, "</body>")
		withStyle = html[:idx] + styleTag + "\n" + html[idx:]
	default:
		withStyle = styleTag + "\n" + html
	}

	opts := premailer.NewOptions()
	opts.RemoveClasses = false
	prem, err := premailer.NewPremailerFromString(withStyle, opts)
	if err != nil {
		return withStyle
	}
	inlined, err := prem.Transform()
	if err != nil {
		return withStyle
	}
	return inlined
}

func renderText(name, tmplStr string, data okapi.M, missingKey string) (string, error) {
	t, err := texttmpl.New(name).
		Option("missingkey=" + missingKey).
		Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("template parse error: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template execute error: %w", err)
	}
	return buf.String(), nil
}

func renderHTML(name, tmplStr string, data okapi.M, missingKey string) (string, error) {
	t, err := htmltmpl.New(name).
		Option("missingkey=" + missingKey).
		Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("template parse error: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template execute error: %w", err)
	}
	return buf.String(), nil
}
