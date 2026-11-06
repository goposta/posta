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
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/storage/repositories"
)

func submissionSchema() models.FieldDefs {
	return models.FieldDefs{
		{Key: "email", Label: "Email", Type: models.FieldEmail, PII: true},
		{Key: "dept", Label: "Department", Type: models.FieldSelect, Options: []models.FieldOption{{Value: "sales"}}},
		{Key: "topics", Label: "Topics", Type: models.FieldMultiSelect, Options: []models.FieldOption{{Value: "api"}}},
		{Key: "nps", Label: "NPS", Type: models.FieldScale},
		{Key: "agree", Label: "Agree", Type: models.FieldCheckbox},
		{Key: "day", Label: "Day", Type: models.FieldDate},
		{Key: "old", Label: "Old", Type: models.FieldText, Archived: true},
	}
}

func TestParseFieldFiltersTypesValues(t *testing.T) {
	q := url.Values{
		"field.dept":     {"sales"},
		"field.topics":   {"api"},
		"field.nps":      {"9"},
		"field.agree":    {"yes"},
		"field.nps[gte]": {"7"},
		"field.day[lte]": {"2026-12-31"},
		"field.old":      {"x"}, // archived questions still filter old answers
		"page":           {"2"},
	}
	equals, ranges, err := parseFieldFilters(submissionSchema(), q)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"dept": "sales", "topics": []any{"api"}, "nps": 9.0, "agree": true, "old": "x"}
	if !reflect.DeepEqual(equals, want) {
		t.Errorf("equals = %#v, want %#v", equals, want)
	}
	if len(ranges) != 2 {
		t.Fatalf("ranges = %+v", ranges)
	}
	byKey := map[string]repositories.DataRange{}
	for _, r := range ranges {
		byKey[r.Key] = r
	}
	if r := byKey["nps"]; !r.Numeric || r.Op != "gte" || r.Value != 7.0 {
		t.Errorf("nps range = %+v", r)
	}
	if r := byKey["day"]; r.Numeric || r.Op != "lte" || r.Value != "2026-12-31" {
		t.Errorf("day range = %+v", r)
	}
}

func TestParseFieldFiltersRejects(t *testing.T) {
	cases := map[string]url.Values{
		"unknown field":               {"field.nope": {"1"}},
		"hostile key":                 {"field.dept') OR 1=1 --": {"x"}},
		"not a number":                {"field.nps": {"high"}},
		"not true or false":           {"field.agree": {"maybe"}},
		"cannot be filtered by range": {"field.dept[gte]": {"a"}},
		"unknown operator":            {"field.nps[ne]": {"1"}},
	}
	for want, q := range cases {
		_, _, err := parseFieldFilters(submissionSchema(), q)
		if err == nil {
			t.Errorf("%v: accepted", q)
			continue
		}
		if want != "hostile key" && !strings.Contains(err.Error(), want) {
			t.Errorf("%v: error %q, want it to mention %q", q, err, want)
		}
	}
	if _, _, err := parseFieldFilters(nil, url.Values{"field.a": {"b"}}); err == nil || !strings.Contains(err.Error(), "no questions") {
		t.Errorf("schemaless form: %v", err)
	}
	if eq, r, err := parseFieldFilters(nil, url.Values{"q": {"x"}}); err != nil || eq != nil || r != nil {
		t.Errorf("no field filters: %v %v %v", eq, r, err)
	}
}

func TestCSVSafe(t *testing.T) {
	for in, want := range map[string]string{
		"=HYPERLINK(\"x\")": "'=HYPERLINK(\"x\")",
		"+1 555":            "'+1 555",
		"-2":                "'-2",
		"@SUM(A1)":          "'@SUM(A1)",
		"\tcmd":             "'\tcmd",
		"plain":             "plain",
		"":                  "",
	} {
		if got := csvSafe(in); got != want {
			t.Errorf("csvSafe(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExportColumnsOrderAndPII(t *testing.T) {
	keys := func(cols []models.FieldDef) []string {
		out := make([]string, 0, len(cols))
		for _, c := range cols {
			out = append(out, c.Key)
		}
		return out
	}
	cols, withheld := exportColumns(submissionSchema(), false)
	if withheld != 1 || !reflect.DeepEqual(keys(cols), []string{"dept", "topics", "nps", "agree", "day", "old"}) {
		t.Errorf("without PII: %v, withheld %d", keys(cols), withheld)
	}
	cols, withheld = exportColumns(submissionSchema(), true)
	if withheld != 0 || keys(cols)[0] != "email" || keys(cols)[len(cols)-1] != "old" {
		t.Errorf("with PII: %v, withheld %d", keys(cols), withheld)
	}
}

func TestOtherFieldsSkipsSchemaAndReserved(t *testing.T) {
	m := &models.Message{FieldsJSON: `[{"key":"dept","value":"sales"},{"key":"utm","value":"ad"},{"key":"_nonce","value":"n"},{"key":"_gotcha","value":""}]`}
	got := otherFields(m, map[string]bool{"dept": true, "_gotcha": true})
	if got != `{"utm":"ad"}` {
		t.Errorf("otherFields = %s", got)
	}
}

// JSON numbers must reach Data and the display copy unchanged; trimming zeros would turn 30 into 3.
func TestStringifyKeepsNumbers(t *testing.T) {
	for in, want := range map[float64]string{
		30:     "30",
		100:    "100",
		10:     "10",
		0:      "0",
		-20:    "-20",
		2.5:    "2.5",
		0.1:    "0.1",
		1e21:   "1000000000000000000000",
		1234.0: "1234",
	} {
		if got := stringify(in); got != want {
			t.Errorf("stringify(%v) = %q, want %q", in, got, want)
		}
	}
	if got := stringify([]any{10.0, "a", true}); got != "10, a, true" {
		t.Errorf("stringify(list) = %q", got)
	}
}
