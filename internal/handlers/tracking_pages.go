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
	"html/template"

	"github.com/jkaninda/okapi"
)

type noticeData struct {
	Title   string
	Heading string
	Message string
	Variant string // "brand" | "success" | "error"
}

func (n noticeData) HeadingText() string {
	if n.Heading != "" {
		return n.Heading
	}
	return n.Title
}

// unsubData drives the unsubscribe confirmation page (with the POST form).
type unsubData struct {
	Title       string
	Heading     string
	Recipient   string
	Message     string
	ButtonLabel string
	Fine        string
}

// webViewData drives the hosted email page.
type webViewData struct {
	Subject string
	Body    template.HTML
}

func trackingHTMLView(c *okapi.Context, status int, page string, data any) error {
	return c.HTMLView(status, trackingPageTemplates+`{{template "`+page+`" .}}`, data)
}

// trackingNotice renders an error/info notice card to the response.
func trackingNotice(c *okapi.Context, status int, title, message, variant string) error {
	return trackingHTMLView(c, status, "notice", noticeData{
		Title:   title,
		Message: message,
		Variant: variant,
	})
}

const trackingPageTemplates = `
{{define "head"}}<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex, nofollow">
<meta name="color-scheme" content="light dark">
<title>{{.Title}}</title>
<style>
  *{box-sizing:border-box}
  body{margin:0;min-height:100vh;display:flex;align-items:center;justify-content:center;padding:24px;
    font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;
    background:#eef0f5;color:#374151;line-height:1.6;-webkit-font-smoothing:antialiased}
  .card{background:#fff;border-radius:16px;padding:44px 36px;max-width:460px;width:100%;text-align:center;
    box-shadow:0 1px 2px rgba(16,24,40,.04),0 10px 28px rgba(16,24,40,.08)}
  .badge{display:inline-flex;align-items:center;justify-content:center;width:60px;height:60px;border-radius:16px;margin-bottom:22px}
  .badge svg{width:30px;height:30px}
  .badge-brand{background:#f3e8ff;color:#7c3aed}
  .badge-success{background:#dcfce7;color:#16a34a}
  .badge-error{background:#fee2e2;color:#dc2626}
  h1{font-size:22px;font-weight:700;color:#111827;margin:0 0 10px;letter-spacing:-.01em}
  p{font-size:15px;color:#6b7280;margin:0 0 6px}
  .recipient{display:inline-block;margin:4px 0 4px;font-weight:600;color:#111827;
    background:#f3f4f6;border-radius:8px;padding:6px 12px;font-size:14px;word-break:break-all}
  form{margin:26px 0 4px}
  .btn{display:inline-block;border:none;cursor:pointer;font-size:15px;font-weight:600;
    padding:13px 32px;border-radius:10px;text-decoration:none;transition:background .15s}
  .btn-primary{background:#7c3aed;color:#fff}
  .btn-primary:hover{background:#6d28d9}
  .fine{font-size:13px;color:#9ca3af;margin-top:16px}
  @media (prefers-color-scheme:dark){
    body{background:#0f1115;color:#cbd2dd}
    .card{background:#1a1d24;box-shadow:none}
    h1{color:#f3f4f6}
    .recipient{background:#2a2e38;color:#f3f4f6}
  }
</style>
</head>
{{end}}

{{define "icon"}}
{{if eq . "success"}}<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"/></svg>
{{else if eq . "error"}}<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="9"/><path d="M12 8v4.5M12 16h.01"/></svg>
{{else}}<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="5" width="18" height="14" rx="2"/><path d="m3.5 7 8.5 6 8.5-6"/></svg>{{end}}
{{end}}

{{define "notice"}}{{template "head" .}}
<body>
  <div class="card">
    <div class="badge badge-{{if .Variant}}{{.Variant}}{{else}}brand{{end}}">{{template "icon" .Variant}}</div>
    <h1>{{.HeadingText}}</h1>
    {{if .Message}}<p>{{.Message}}</p>{{end}}
  </div>
</body>
</html>{{end}}

{{define "unsubscribe"}}{{template "head" .}}
<body>
  <div class="card">
    <div class="badge badge-brand">{{template "icon" "brand"}}</div>
    <h1>{{.Heading}}</h1>
    {{if .Message}}<p>{{.Message}}</p>{{end}}
    {{if .Recipient}}<span class="recipient">{{.Recipient}}</span>{{end}}
    <form method="POST">
      <button type="submit" class="btn btn-primary">{{.ButtonLabel}}</button>
    </form>
    {{if .Fine}}<p class="fine">{{.Fine}}</p>{{end}}
  </div>
</body>
</html>{{end}}

{{define "webview"}}<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex, nofollow">
<title>{{.Subject}}</title>
<style>
  body{margin:0;background:#eef0f5;color:#374151;
    font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif}
  .wv-bar{position:sticky;top:0;z-index:10;display:flex;align-items:center;gap:12px;
    padding:12px 18px;background:#fff;border-bottom:1px solid #e5e7eb;box-shadow:0 1px 2px rgba(16,24,40,.04)}
  .wv-mark{flex:none;width:34px;height:34px;border-radius:9px;background:#7c3aed;color:#fff;
    display:flex;align-items:center;justify-content:center}
  .wv-mark svg{width:18px;height:18px}
  .wv-meta{min-width:0}
  .wv-label{font-size:11px;text-transform:uppercase;letter-spacing:.06em;color:#9ca3af;font-weight:600}
  .wv-subject{font-size:14px;font-weight:600;color:#111827;
    white-space:nowrap;overflow:hidden;text-overflow:ellipsis;max-width:60vw}
  .wv-wrap{max-width:720px;margin:24px auto;padding:0 16px}
  .wv-sheet{background:#fff;border:1px solid #e5e7eb;border-radius:14px;overflow:hidden;
    box-shadow:0 1px 2px rgba(16,24,40,.04),0 8px 24px rgba(16,24,40,.06)}
  .wv-footer{text-align:center;font-size:12px;color:#9ca3af;padding:18px 16px 28px}
</style>
</head>
<body>
  <div class="wv-bar">
    <div class="wv-mark"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="5" width="18" height="14" rx="2"/><path d="m3.5 7 8.5 6 8.5-6"/></svg></div>
    <div class="wv-meta">
      <div class="wv-label">Viewing in browser</div>
      <div class="wv-subject">{{.Subject}}</div>
    </div>
  </div>
  <div class="wv-wrap">
    <div class="wv-sheet">{{.Body}}</div>
  </div>
  <div class="wv-footer">This is a web copy of an email you received.</div>
</body>
</html>{{end}}
`
