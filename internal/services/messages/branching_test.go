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

package messages

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/goposta/posta/internal/models"
)

// An application form: individuals skip the company page, and anyone who
// says they are not hiring ends early.
func branchingForm() models.FieldDefs {
	return models.FieldDefs{
		{Key: "email", Label: "Email", Type: models.FieldEmail, Required: true},
		{Key: "kind", Label: "I am", Type: models.FieldSelect, Required: true, Options: []models.FieldOption{
			{Value: "company", Label: "A company"},
			{Value: "individual", Label: "An individual", Goto: "about"},
			{Value: "browsing", Label: "Just looking", Goto: models.GotoSubmit},
		}},
		{Key: "utm", Type: models.FieldHidden},
		{Key: "company", Label: "Company details", Type: models.FieldSection, Next: "wrapup"},
		{Key: "company_name", Label: "Company name", Type: models.FieldText, Required: true},
		{Key: "about", Label: "About you", Type: models.FieldSection},
		{Key: "bio", Label: "Bio", Type: models.FieldTextarea, Required: true},
		{Key: "wrapup", Label: "Last step", Type: models.FieldSection},
		{Key: "notes", Label: "Anything else?", Type: models.FieldTextarea},
	}
}

func routeKeys(defs models.FieldDefs, answers map[string]string) []string {
	var out []string
	for k := range Route(defs, func(key string) string { return answers[key] }) {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestRoute(t *testing.T) {
	defs := branchingForm()
	cases := []struct {
		kind string
		want []string
	}{
		// The company page's own next skips "about you".
		{"company", []string{"company_name", "email", "kind", "notes", "utm"}},
		{"individual", []string{"bio", "email", "kind", "notes", "utm"}},
		{"browsing", []string{"email", "kind", "utm"}},
		// No answer yet: pages follow in order, each page's next applying.
		{"", []string{"company_name", "email", "kind", "notes", "utm"}},
	}
	for _, c := range cases {
		if got := routeKeys(defs, map[string]string{"kind": c.kind}); !reflect.DeepEqual(got, c.want) {
			t.Errorf("kind=%q reached %v, want %v", c.kind, got, c.want)
		}
	}

	plain := models.FieldDefs{{Key: "a", Label: "A", Type: models.FieldText}, {Key: "b", Label: "B", Type: models.FieldText}}
	if got := routeKeys(plain, nil); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("a form without sections reached %v", got)
	}

	// An archived section folds its questions into the page before it: "bio"
	// joins the company page.
	archived := branchingForm()
	archived[5].Archived = true
	if got := routeKeys(archived, map[string]string{"kind": "company"}); !reflect.DeepEqual(got, []string{"bio", "company_name", "email", "kind", "notes", "utm"}) {
		t.Errorf("with 'about' archived, company reached %v", got)
	}
}

func TestPages(t *testing.T) {
	pages := Pages(branchingForm())
	if len(pages) != 4 || pages[0].Section != nil || pages[1].Section.Key != "company" {
		t.Fatalf("got %d pages", len(pages))
	}
	opening := models.FieldDefs{{Key: "intro", Label: "Intro", Type: models.FieldSection}, {Key: "a", Label: "A", Type: models.FieldText}}
	if p := Pages(opening); len(p) != 1 || p[0].Section == nil || p[0].Section.Key != "intro" {
		t.Errorf("a form opening with a section should have it as its only page, got %+v", p)
	}
}

func TestValidateFollowsRoute(t *testing.T) {
	form := &models.Form{Fields: branchingForm()}

	// An individual never sees the required company name, and answers sent
	// for a skipped page are not stored.
	got, errs := Validate(form, fieldsOf("email", "ada@example.com", "kind", "individual", "bio", "Hi", "company_name", "Acme"))
	if errs != nil {
		t.Fatalf("individual rejected: %+v", errs)
	}
	if _, kept := got.Data["company_name"]; kept {
		t.Error("an answer on a skipped page was stored")
	}
	for _, f := range got.Fields {
		if f.Key == "company_name" {
			t.Error("an answer on a skipped page is in the display copy")
		}
	}

	// A company must give its name, and its bio is not asked.
	_, errs = Validate(form, fieldsOf("email", "ada@example.com", "kind", "company"))
	if len(errs) != 1 || errs[0].Key != "company_name" || errs[0].Code != "required" {
		t.Errorf("company without a name: %+v", errs)
	}

	if _, errs := Validate(form, fieldsOf("email", "ada@example.com", "kind", "browsing")); errs != nil {
		t.Errorf("ending early rejected: %+v", errs)
	}
	// A section key sent as a field is ignored, never stored.
	got, errs = Validate(form, fieldsOf("email", "ada@example.com", "kind", "browsing", "company", "x"))
	if errs != nil || got.Data["company"] != nil {
		t.Errorf("section key as a field: errs=%+v data=%v", errs, got.Data)
	}
}

func TestCheckSchemaFlow(t *testing.T) {
	if err := CheckSchema(branchingForm(), nil, "_gotcha"); err != nil {
		t.Fatalf("valid branching form rejected: %v", err)
	}
	mutate := func(fn func(d models.FieldDefs) models.FieldDefs) models.FieldDefs { return fn(branchingForm()) }
	cases := []struct {
		name string
		defs models.FieldDefs
		want string
	}{
		{"goto backwards", mutate(func(d models.FieldDefs) models.FieldDefs {
			d[6].Type, d[6].Options = models.FieldSelect, []models.FieldOption{{Value: "x", Goto: "company"}}
			return d
		}), "later section"},
		{"goto unknown", mutate(func(d models.FieldDefs) models.FieldDefs {
			d[1].Options[1].Goto = "nowhere"
			return d
		}), "later section"},
		{"next backwards", mutate(func(d models.FieldDefs) models.FieldDefs { d[7].Next = "about"; return d }), "later section"},
		{"goto on multiselect", mutate(func(d models.FieldDefs) models.FieldDefs { d[1].Type = models.FieldMultiSelect; return d }), "single-choice"},
		{"two branches on a page", mutate(func(d models.FieldDefs) models.FieldDefs {
			extra := models.FieldDef{Key: "kind2", Label: "Again", Type: models.FieldSelect, Options: []models.FieldOption{{Value: "y", Goto: "wrapup"}}}
			return append(d[:2], append(models.FieldDefs{extra}, d[2:]...)...)
		}), "same page"},
		{"goto archived section", mutate(func(d models.FieldDefs) models.FieldDefs { d[5].Archived = true; return d }), "later section"},
		{"section with question settings", mutate(func(d models.FieldDefs) models.FieldDefs { d[3].Required = true; return d }), "only takes"},
		{"section keyed submit", mutate(func(d models.FieldDefs) models.FieldDefs { d[7].Key = "submit"; return d }), "cannot be a section key"},
		{"section without title", mutate(func(d models.FieldDefs) models.FieldDefs { d[3].Label = ""; return d }), "title"},
		{"next on a question", mutate(func(d models.FieldDefs) models.FieldDefs { d[0].Next = "about"; return d }), "only a section"},
	}
	for _, c := range cases {
		err := CheckSchema(c.defs, nil, "_gotcha")
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want it to mention %q", c.name, err, c.want)
		}
	}
}
