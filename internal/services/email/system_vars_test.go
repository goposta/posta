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
	"strings"
	"testing"

	"github.com/jkaninda/okapi"
)

func TestWithSystemVarsAddsReservedKeysAndDoesNotMutateInput(t *testing.T) {
	in := map[string]any{"name": "Ada"}
	out := WithSystemVars(in)

	for _, k := range []string{"posta_web_view_url", "posta_mail_web_link", "posta_unsubscribe_url"} {
		if _, ok := out[k]; !ok {
			t.Fatalf("expected reserved key %q to be present", k)
		}
	}
	if out["name"] != "Ada" {
		t.Fatalf("user data lost: %v", out["name"])
	}
	if _, ok := in["posta_web_view_url"]; ok {
		t.Fatalf("input map must not be mutated")
	}
}

func TestSystemVarsUserCannotShadow(t *testing.T) {
	out := WithSystemVars(map[string]any{"posta_web_view_url": "https://evil.example"})
	if out["posta_web_view_url"] == "https://evil.example" {
		t.Fatalf("user value must not override reserved system variable")
	}
}

func TestRenderThenSubstituteSystemLinks(t *testing.T) {
	r := NewTemplateRenderer()
	r.MissingKeyBehavior = "error"

	// missingkey=error must not fire for reserved vars because WithSystemVars always supplies them.
	in := &RenderInput{
		HTMLTemplate: `<a href="{{ posta_web_view_url }}">view</a> <a href="{{ posta_unsubscribe_url }}">unsub</a>`,
		TextTemplate: `view: {{ posta_mail_web_link }}`,
	}
	rendered, err := r.Render(in, WithSystemVars(okapi.M{}))
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	if !HasSystemSentinels(rendered.HTML) || !HasSystemSentinels(rendered.Text) {
		t.Fatalf("expected sentinels to survive rendering; got html=%q text=%q", rendered.HTML, rendered.Text)
	}

	html := SubstituteSystemLinks(rendered.HTML, "https://p/t/v/tok", "https://p/t/u/tx/tok")
	if strings.Contains(html, sentinelWebView) || strings.Contains(html, sentinelUnsubscribe) {
		t.Fatalf("sentinels not fully replaced: %q", html)
	}
	if !strings.Contains(html, "https://p/t/v/tok") || !strings.Contains(html, "https://p/t/u/tx/tok") {
		t.Fatalf("real links missing after substitution: %q", html)
	}

	text := SubstituteSystemLinks(rendered.Text, "https://p/t/v/tok", "x")
	if !strings.Contains(text, "view: https://p/t/v/tok") {
		t.Fatalf("text alias not substituted: %q", text)
	}
}

func TestWithSystemVarNamesResolvesToNames(t *testing.T) {
	r := NewTemplateRenderer()
	r.MissingKeyBehavior = "error"

	in := &RenderInput{HTMLTemplate: `<a href="{{ posta_web_view_url }}">v</a>{{ posta_unsubscribe_url }}`}
	// missingkey=error must not fire, and values must be the variable names.
	rendered, err := r.Render(in, WithSystemVarNames(map[string]any{}))
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	if !strings.Contains(rendered.HTML, VarWebView) || !strings.Contains(rendered.HTML, VarUnsubscribe) {
		t.Fatalf("expected variable names in output, got %q", rendered.HTML)
	}
	if HasSystemSentinels(rendered.HTML) {
		t.Fatalf("name-mode preview must not contain sentinels: %q", rendered.HTML)
	}
}

func TestSystemVarNames(t *testing.T) {
	names := SystemVarNames()
	want := map[string]bool{VarWebView: false, VarWebViewAlias: false, VarUnsubscribe: false}
	for _, n := range names {
		if _, ok := want[n]; !ok {
			t.Fatalf("unexpected name %q", n)
		}
		want[n] = true
	}
	for n, seen := range want {
		if !seen {
			t.Fatalf("missing name %q", n)
		}
	}
}

func TestSubstituteSystemLinksEmptyRemovesSentinel(t *testing.T) {
	got := SubstituteSystemLinks(`<a href="`+sentinelWebView+`">x</a>`, "", "")
	if strings.Contains(got, sentinelWebView) {
		t.Fatalf("empty url should remove sentinel, got %q", got)
	}
}
