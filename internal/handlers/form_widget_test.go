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
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/web/widget"
	"github.com/jkaninda/okapi"
)

func TestWidgetConfig(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	past, future := now.Add(-time.Hour), now.Add(time.Hour)
	active := func(mut func(*models.Form)) *models.Form {
		f := &models.Form{Status: models.FormStatusActive, PublicKey: "k"}
		if mut != nil {
			mut(f)
		}
		return f
	}
	// request builds a request from origin to Posta at posta.test; an origin
	// of "self-get" is a same-origin GET, which carries no Origin header.
	request := func(origin string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "https://posta.test/api/v1/f/k/config", nil)
		switch origin {
		case "self-get":
			r.Header.Set("Sec-Fetch-Site", "same-origin")
		case "":
		default:
			r.Header.Set("Origin", origin)
		}
		return r
	}
	open := WidgetConfig{Enabled: true, HoneypotField: "_gotcha"}
	cases := []struct {
		name   string
		form   *models.Form
		origin string
		want   WidgetConfig
	}{
		{"unknown key", nil, "https://site.test", WidgetConfig{Reason: "unavailable"}},
		{"archived", active(func(f *models.Form) { f.Status = models.FormStatusArchived }), "https://site.test", WidgetConfig{Reason: "unavailable"}},
		{"paused is closed", active(func(f *models.Form) { f.Status = models.FormStatusPaused }), "https://site.test", WidgetConfig{Reason: "closed", ClosedMessage: defaultClosedMessage}},
		{"past close_at", active(func(f *models.Form) { f.CloseAt = &past; f.ClosedMessage = "See you next year" }), "https://site.test", WidgetConfig{Reason: "closed", ClosedMessage: "See you next year"}},
		{"future close_at", active(func(f *models.Form) { f.CloseAt = &future }), "https://site.test", open},
		{"response limit reached", active(func(f *models.Form) { f.MaxResponses = 10; f.MessageCount = 12; f.SpamCount = 2 }), "https://site.test", WidgetConfig{Reason: "closed", ClosedMessage: defaultClosedMessage}},
		{"spam does not count", active(func(f *models.Form) { f.MaxResponses = 10; f.MessageCount = 12; f.SpamCount = 3 }), "https://site.test", open},
		{"open allowlist", active(nil), "https://site.test", open},
		{"listed origin", active(func(f *models.Form) { f.AllowedOrigins = []string{"https://site.test"} }), "https://site.test", open},
		{"unlisted origin", active(func(f *models.Form) { f.AllowedOrigins = []string{"https://other.test"} }), "https://site.test", WidgetConfig{Reason: "origin_not_allowed"}},
		{"strict without origin", active(func(f *models.Form) { f.StrictOrigin = true }), "", WidgetConfig{Reason: "origin_not_allowed"}},
		{"hosted page post", active(func(f *models.Form) { f.AllowedOrigins = []string{"https://other.test"}; f.StrictOrigin = true }), "https://posta.test", open},
		{"hosted page get", active(func(f *models.Form) { f.AllowedOrigins = []string{"https://other.test"}; f.StrictOrigin = true }), "self-get", open},
		{"custom honeypot and nonce", active(func(f *models.Form) { f.HoneypotField = "website"; f.RequireNonce = true; f.MinFillSeconds = 3 }), "https://site.test", WidgetConfig{Enabled: true, HoneypotField: "website", RequireNonce: true, MinFillSeconds: 3}},
		{"fill time only with a nonce", active(func(f *models.Form) { f.MinFillSeconds = 3 }), "https://site.test", open},
		{"responses and appearance", active(func(f *models.Form) {
			f.ConfirmationMessage, f.AllowAnotherResponse = "Thanks!", true
			f.Appearance = models.FormAppearance{Accent: "#0f766e", Theme: "dark"}
		}), "https://site.test", WidgetConfig{Enabled: true, HoneypotField: "_gotcha", ConfirmationMessage: "Thanks!", AllowAnotherResponse: true, Appearance: &models.FormAppearance{Accent: "#0f766e", Theme: "dark"}}},
	}
	for _, tc := range cases {
		originOK := tc.form != nil && originAllowed(request(tc.origin), tc.form)
		if got := widgetConfig(tc.form, originOK, now); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: widgetConfig = %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

func TestFormScriptHeaders(t *testing.T) {
	h := NewFormIngestHandler(nil, nil, 0)
	serve := func(req *http.Request) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		if err := h.FormScript(okapi.NewContext(okapi.New(), rec, req)); err != nil {
			t.Fatalf("FormScript: %v", err)
		}
		return rec
	}

	rec := serve(httptest.NewRequest(http.MethodGet, "/widget/v1/posta-form.js", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	for header, want := range map[string]string{
		"Content-Type":                 "text/javascript; charset=utf-8",
		"X-Content-Type-Options":       "nosniff",
		"Cross-Origin-Resource-Policy": "cross-origin",
		"Cache-Control":                "public, max-age=3600",
		"ETag":                         widget.FormScriptETag,
	} {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	if rec.Body.Len() != len(widget.FormScript) {
		t.Fatalf("body is %d bytes, want %d", rec.Body.Len(), len(widget.FormScript))
	}

	req := httptest.NewRequest(http.MethodGet, "/widget/v1/posta-form.js", nil)
	req.Header.Set("If-None-Match", widget.FormScriptETag)
	if rec := serve(req); rec.Code != http.StatusNotModified {
		t.Fatalf("revalidation status = %d, want 304", rec.Code)
	}
}

func TestBuildWidgetSnippet(t *testing.T) {
	got := buildWidgetSnippet("https://posta.example.com", &models.Form{PublicKey: "abc123"})
	for _, want := range []string{
		`<div data-posta-form="abc123"></div>`,
		`<script src="https://posta.example.com/widget/v1/posta-form.js" async></script>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("snippet is missing %q:\n%s", want, got)
		}
	}
}
