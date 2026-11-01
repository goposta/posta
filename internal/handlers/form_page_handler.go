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
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/settings"
	"github.com/jkaninda/okapi"
)

const maxPrefillLength = 2000

// fieldEmail is the key of an email field: the widget's built-in field and
// the email column of a subscriber import.
const fieldEmail = "email"

// builtinPrefillKeys are the fields the widget shows for a form without a
// schema, so its share link can prefill them too.
var builtinPrefillKeys = []string{"name", fieldEmail, "phone", "subject", "message"}

// hostedPageCSP keeps a hosted page to Posta's own script and API. Inline
// styles are allowed for the page and the widget's shadow root; images may
// come from any https host for the header image.
const hostedPageCSP = "default-src 'none'; script-src 'self'; connect-src 'self'; " +
	"style-src 'self' 'unsafe-inline'; img-src https: data:; form-action 'none'; " +
	"frame-ancestors 'none'; base-uri 'none'"

// SetHostedPages enables the hosted form pages. adminEmail is the abuse
// contact when the platform setting abuse_report_email is empty.
func (h *FormIngestHandler) SetHostedPages(p *settings.Provider, adminEmail string) {
	h.settings = p
	h.abuseFallback = adminEmail
}

type hostedPageData struct {
	Title       string
	Description string
	Key         string
	Theme       string
	HeaderImage string
	Attrs       []template.HTMLAttr
	AbuseMail   string
}

// HostedPage serves a form's share link: a page on Posta's own domain that
// shows the form through the website widget.
func (h *FormIngestHandler) HostedPage(c *okapi.Context) error {
	if h.settings == nil || !h.settings.HostedFormsEnabled() {
		return trackingNotice(c, http.StatusNotFound, "Form not found", "This form does not exist.", "error")
	}
	key := c.Param("key")
	form, err := h.svc.FindFormByKey(key)
	if err != nil || form.Status == models.FormStatusArchived {
		return trackingNotice(c, http.StatusNotFound, "Form not found", "This form does not exist.", "error")
	}

	data := hostedPageData{
		Title:       form.Name,
		Description: form.Description,
		Key:         form.PublicKey,
		Theme:       form.Appearance.Theme,
		HeaderImage: form.Appearance.HeaderImageURL,
		Attrs:       prefillAttrs(form, c.Request().URL.Query()),
		AbuseMail:   abuseMailto(h.abuseContact(h.settings.AbuseReportEmail()), form.PublicKey),
	}
	if data.Theme == "" {
		data.Theme = "light"
	}

	hdr := c.ResponseWriter().Header()
	hdr.Set("Content-Type", "text/html; charset=utf-8")
	hdr.Set("Content-Security-Policy", hostedPageCSP)
	hdr.Set("X-Content-Type-Options", "nosniff")
	hdr.Set("Referrer-Policy", "no-referrer")
	hdr.Set("Cache-Control", "no-store")
	c.ResponseWriter().WriteHeader(http.StatusOK)
	return hostedPageTemplate.Execute(c.ResponseWriter(), data)
}

// abuseContact is the configured abuse address, or the admin's.
func (h *FormIngestHandler) abuseContact(configured string) string {
	if c := strings.TrimSpace(configured); c != "" {
		return c
	}
	return h.abuseFallback
}

func abuseMailto(contact, formKey string) string {
	if contact == "" {
		return ""
	}
	q := url.Values{}
	q.Set("subject", "Abuse report: form "+formKey)
	return "mailto:" + contact + "?" + strings.ReplaceAll(q.Encode(), "+", "%20")
}

// prefillAttrs turns ?<key>=value into data-value-<key> attributes for the
// widget. Only declared, non-PII fields are prefilled: a link must not be a
// way to plant personal data in someone else's submission. Keys come from the
// schema (lowercase letters, digits, _), so they are safe attribute names.
func prefillAttrs(form *models.Form, query url.Values) []template.HTMLAttr {
	var attrs []template.HTMLAttr
	add := func(key, value string) {
		if len(value) > maxPrefillLength {
			return
		}
		attrs = append(attrs, template.HTMLAttr(fmt.Sprintf(`data-value-%s="%s"`, key, html.EscapeString(value))))
	}
	if !form.HasSchema() {
		for _, key := range builtinPrefillKeys {
			if v := query.Get(key); v != "" {
				add(key, v)
			}
		}
		return attrs
	}
	for _, f := range form.Fields.Active() {
		values := query[f.Key]
		if f.PII || len(values) == 0 {
			continue
		}
		if f.Type == models.FieldMultiSelect {
			encoded, _ := json.Marshal(values)
			add(f.Key, string(encoded))
			continue
		}
		add(f.Key, values[0])
	}
	return attrs
}

var hostedPageTemplate = template.Must(template.New("page").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex, nofollow">
<title>{{.Title}}</title>
<style>
:root{--bg:#F2F4FA;--fg:#101A37;--mut:#5A6486}
:root[data-theme=dark]{--bg:#0A1124;--fg:#F2F4FA;--mut:#A9B1CC}
@media (prefers-color-scheme:dark){:root[data-theme=auto]{--bg:#0A1124;--fg:#F2F4FA;--mut:#A9B1CC}}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--fg);font:16px/1.5 system-ui,-apple-system,"Segoe UI",sans-serif}
main{max-width:680px;margin:0 auto;padding:32px 16px 24px}
.header{display:block;width:100%;max-height:220px;object-fit:cover;border-radius:12px;margin:0 0 20px}
[data-posta-form]{--posta-form-max-width:100%}
footer{max-width:680px;margin:0 auto;padding:0 16px 32px;font-size:13px;color:var(--mut);text-align:center}
footer a{color:inherit}
</style>
</head>
<body>
<main>
{{if .HeaderImage}}<img class="header" src="{{.HeaderImage}}" alt="">{{end}}
<div data-posta-form="{{.Key}}" data-title="{{.Title}}"{{if .Description}} data-description="{{.Description}}"{{end}} data-theme="{{.Theme}}"{{range .Attrs}} {{.}}{{end}}></div>
</main>
<footer>
Never submit passwords through Posta forms.{{if .AbuseMail}} · <a href="{{.AbuseMail}}">Report abuse</a>{{end}}
</footer>
<script src="/widget/v1/posta-form.js" async></script>
</body>
</html>
`))
