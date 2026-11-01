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
	"bytes"
	"net/url"
	"strings"
	"testing"

	"github.com/goposta/posta/internal/models"
)

func TestPrefillAttrs(t *testing.T) {
	form := &models.Form{Fields: models.FieldDefs{
		{Key: "plan", Label: "Plan", Type: models.FieldSelect, Options: []models.FieldOption{{Value: "pro"}}},
		{Key: "topics", Label: "Topics", Type: models.FieldMultiSelect, Options: []models.FieldOption{{Value: "a, b"}, {Value: "c"}}},
		{Key: "email", Label: "Email", Type: models.FieldEmail, PII: true},
		{Key: "old", Label: "Old", Type: models.FieldText, Archived: true},
		{Key: "note", Label: "Note", Type: models.FieldText},
	}}
	query := url.Values{
		"plan":    {"pro"},
		"topics":  {"a, b", "c"},
		"email":   {"victim@example.com"},
		"old":     {"x"},
		"unknown": {"x"},
		"note":    {strings.Repeat("x", maxPrefillLength+1)},
	}
	var got []string
	for _, a := range prefillAttrs(form, query) {
		got = append(got, string(a))
	}
	want := []string{`data-value-plan="pro"`, `data-value-topics="[&#34;a, b&#34;,&#34;c&#34;]"`}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("prefill attrs:\n got %v\nwant %v (PII, archived, unknown and overlong values must be skipped)", got, want)
	}

	schemaless := prefillAttrs(&models.Form{}, url.Values{"name": {"Ada"}, "other": {"x"}})
	if len(schemaless) != 1 || string(schemaless[0]) != `data-value-name="Ada"` {
		t.Errorf("schemaless prefill = %v, want only the built-in name field", schemaless)
	}
}

func TestHostedPageEscapesFormText(t *testing.T) {
	var buf bytes.Buffer
	err := hostedPageTemplate.Execute(&buf, hostedPageData{
		Title:       `</title><script>alert(1)</script>`,
		Description: `" onmouseover="alert(1)`,
		Key:         "abc",
		Theme:       "light",
		HeaderImage: "javascript:alert(1)",
		Attrs:       prefillAttrs(&models.Form{}, url.Values{"name": {`"><script>alert(1)</script>`}}),
		AbuseMail:   "mailto:abuse@example.com?subject=x",
	})
	if err != nil {
		t.Fatal(err)
	}
	page := buf.String()
	for _, bad := range []string{"<script>alert(1)", `" onmouseover="`, `src="javascript:`} {
		if strings.Contains(page, bad) {
			t.Errorf("page contains unescaped %q", bad)
		}
	}
	for _, want := range []string{
		`<script src="/widget/v1/posta-form.js" async></script>`,
		"Never submit passwords through Posta forms.",
		`href="mailto:abuse@example.com?subject=x"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("page is missing %q", want)
		}
	}
}

func TestAbuseMailtoFallsBackToAdmin(t *testing.T) {
	h := &FormIngestHandler{abuseFallback: "admin@example.com"}
	if got := h.abuseContact(""); got != "admin@example.com" {
		t.Errorf("fallback contact = %q", got)
	}
	if got := h.abuseContact("abuse@example.com"); got != "abuse@example.com" {
		t.Errorf("configured contact = %q", got)
	}
	if got := abuseMailto("abuse@example.com", "KEY1"); got != "mailto:abuse@example.com?subject=Abuse%20report%3A%20form%20KEY1" {
		t.Errorf("mailto = %q", got)
	}
}
