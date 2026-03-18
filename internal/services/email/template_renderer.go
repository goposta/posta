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
	"bytes"
	"fmt"
	htmltmpl "html/template"
	"regexp"
	"strings"
	texttmpl "text/template"

	"github.com/jkaninda/okapi"
	"github.com/vanng822/go-premailer/premailer"
)

// actionRe matches any {{ ... }} action block, including whitespace-trim variants.
var actionRe = regexp.MustCompile(`\{\{(-?)\s*(.*?)\s*(-?)\}\}`)

// identRe matches a bare identifier that needs a dot prefix (no leading dot or $).
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

// normalizeTemplate rewrites bare identifiers inside {{ }} actions to their
// dot-prefixed equivalents so they resolve against the data map instead of
// being looked up as functions.
//
// Rules:
//   - {{ .varName }}          → untouched (already dotted)
//   - {{ $var }}              → untouched (template variable)
//   - {{ varName }}           → {{ .varName }}
//   - {{ range features }}    → {{ range .features }}
//   - {{ if active }}         → {{ if .active }}
//   - {{ end }}, {{ else }}   → untouched (no argument)
//   - {{- varName -}}         → {{- .varName -}} (trim dashes preserved)
func normalizeTemplate(s string) string {
	return actionRe.ReplaceAllStringFunc(s, func(match string) string {
		m := actionRe.FindStringSubmatch(match)
		if m == nil {
			return match
		}
		leftDash := m[1]  // "-" or ""
		inner := m[2]     // content between {{ and }}
		rightDash := m[3] // "-" or ""

		open := "{{" + leftDash
		close := rightDash + "}}"

		// Rebuild with normalized inner content.
		normalized := normalizeParts(inner)
		return open + " " + normalized + " " + close
	})
}

// normalizeParts normalizes the inner content of a template action.
// It handles:
//   - Single tokens: "varName" → ".varName"
//   - Keyword + argument: "range features" → "range .features"
//   - Already-dotted or $-prefixed: left alone
func normalizeParts(inner string) string {
	parts := strings.Fields(inner)
	if len(parts) == 0 {
		return inner
	}

	first := parts[0]

	if len(parts) == 1 {
		return dotPrefix(first)
	}

	// Multi-token: keyword followed by an argument (e.g. "range features")
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

// dotPrefix adds a "." prefix to a bare identifier.
// It leaves alone: already-dotted (.foo), variables ($foo), keywords, and "."
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

type TemplateRenderer struct {
	// MissingKeyBehavior controls how missing data keys are handled.
	// "error" (default): return an error on missing key
	// "zero": silently use zero value
	// "invalid": render "<no value>"
	MissingKeyBehavior string
}

func NewTemplateRenderer() *TemplateRenderer {
	return &TemplateRenderer{
		MissingKeyBehavior: "error",
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
	case "zero", "invalid":
		return r.MissingKeyBehavior
	default:
		return "error"
	}
}

// injectCSS inserts a <style> block into the rendered HTML and inlines styles
// using premailer for maximum email client compatibility.
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

// renderHTML uses html/template for auto-escaping of HTML content.
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
