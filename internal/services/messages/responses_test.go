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
	"testing"

	"github.com/goposta/posta/internal/models"
)

func surveySchema() models.FieldDefs {
	return models.FieldDefs{
		{Key: "email", Label: "Email", Type: models.FieldEmail, PII: true},
		{Key: "dept", Label: "Department", Type: models.FieldSelect, Options: []models.FieldOption{{Value: "sales", Label: "Sales"}, {Value: "support"}}},
		{Key: "topics", Label: "Topics", Type: models.FieldMultiSelect, Options: []models.FieldOption{{Value: "api"}, {Value: "ui", Label: "Dashboard"}}},
		{Key: "nps", Label: "Recommend", Type: models.FieldScale, Min: ptrF(0), Max: ptrF(3)},
		{Key: "stars", Label: "Stars", Type: models.FieldRating, Max: ptrF(3)},
		{Key: "agree", Label: "Agree", Type: models.FieldCheckbox},
		{Key: "seats", Label: "Seats", Type: models.FieldNumber},
		{Key: "notes", Label: "Notes", Type: models.FieldTextarea},
		{Key: "old", Label: "Old", Type: models.FieldText, Archived: true},
		{Key: "src", Type: models.FieldHidden},
	}
}

func counts(fs FieldSummary) map[string]int {
	out := map[string]int{}
	for _, o := range fs.Options {
		out[o.Value] = o.Count
	}
	return out
}

func TestSummarizerCountsEveryQuestionType(t *testing.T) {
	s := NewSummarizer(surveySchema(), false)
	s.Add(models.SubmissionData{"email": "a@x.io", "dept": "sales", "topics": []any{"api", "ui"}, "nps": 3.0, "stars": 2.0, "agree": true, "seats": 10.0, "notes": "newest"})
	s.Add(models.SubmissionData{"email": "b@x.io", "dept": "legacy", "topics": []any{"api"}, "nps": 0.0, "agree": false, "seats": 4.0, "notes": "  "})
	s.Add(models.SubmissionData{"dept": "sales", "old": "kept", "src": "ad"})
	got := s.Result(false)

	if got.Total != 3 {
		t.Fatalf("total = %d, want 3", got.Total)
	}
	keys := make([]string, 0, len(got.Fields))
	byKey := map[string]FieldSummary{}
	for _, f := range got.Fields {
		keys = append(keys, f.Key)
		byKey[f.Key] = f
	}
	// Archived and hidden questions are not summarized.
	if want := []string{"email", "dept", "topics", "nps", "stars", "agree", "seats", "notes"}; !reflect.DeepEqual(keys, want) {
		t.Fatalf("fields = %v, want %v", keys, want)
	}

	dept := byKey["dept"]
	if dept.Answered != 3 || !reflect.DeepEqual(counts(dept), map[string]int{"sales": 2, "support": 0, "legacy": 1}) {
		t.Errorf("dept = %+v", dept)
	}
	// Schema order first, then values the schema no longer lists.
	if order := []string{dept.Options[0].Value, dept.Options[1].Value, dept.Options[2].Value}; !reflect.DeepEqual(order, []string{"sales", "support", "legacy"}) {
		t.Errorf("dept order = %v", order)
	}
	if dept.Options[1].Label != "support" || dept.Options[0].Label != "Sales" {
		t.Errorf("dept labels = %+v", dept.Options)
	}
	if c := counts(byKey["topics"]); c["api"] != 2 || c["ui"] != 1 || byKey["topics"].Answered != 2 {
		t.Errorf("topics = %+v", byKey["topics"])
	}
	nps := byKey["nps"]
	if !reflect.DeepEqual(counts(nps), map[string]int{"0": 1, "1": 0, "2": 0, "3": 1}) || nps.Mean == nil || *nps.Mean != 1.5 {
		t.Errorf("nps = %+v", nps)
	}
	if c := counts(byKey["stars"]); len(byKey["stars"].Options) != 3 || c["2"] != 1 {
		t.Errorf("stars = %+v", byKey["stars"])
	}
	if c := counts(byKey["agree"]); c["true"] != 1 || c["false"] != 1 {
		t.Errorf("agree = %+v", byKey["agree"])
	}
	seats := byKey["seats"]
	if seats.Options != nil || *seats.Mean != 7 || *seats.Min != 4 || *seats.Max != 10 {
		t.Errorf("seats = %+v", seats)
	}
	// Blank answers are not answers.
	if notes := byKey["notes"]; notes.Answered != 1 || !reflect.DeepEqual(notes.Samples, []string{"newest"}) {
		t.Errorf("notes = %+v", notes)
	}
}

func TestSummarizerWithholdsPersonalDataSamples(t *testing.T) {
	data := models.SubmissionData{"email": "a@x.io"}

	hidden := NewSummarizer(surveySchema(), false)
	hidden.Add(data)
	email := hidden.Result(false).Fields[0]
	if email.Samples != nil || !email.SamplesHidden || email.Answered != 1 {
		t.Errorf("without PII access: %+v", email)
	}

	shown := NewSummarizer(surveySchema(), true)
	shown.Add(data)
	email = shown.Result(true).Fields[0]
	if !reflect.DeepEqual(email.Samples, []string{"a@x.io"}) || email.SamplesHidden {
		t.Errorf("with PII access: %+v", email)
	}
	if !shown.Result(true).Truncated {
		t.Error("truncated flag lost")
	}
}

func TestSummarizerKeepsTheLatestSamples(t *testing.T) {
	s := NewSummarizer(models.FieldDefs{{Key: "q", Label: "Q", Type: models.FieldText}}, false)
	for i := 0; i < summarySamples+5; i++ {
		s.Add(models.SubmissionData{"q": string(rune('a' + i))})
	}
	q := s.Result(false).Fields[0]
	if q.Answered != summarySamples+5 || len(q.Samples) != summarySamples || q.Samples[0] != "a" {
		t.Errorf("samples = %d (first %q), answered %d", len(q.Samples), q.Samples[0], q.Answered)
	}
}

func TestExportValue(t *testing.T) {
	dept := &models.FieldDef{Type: models.FieldSelect, Options: []models.FieldOption{{Value: "sales", Label: "Sales"}, {Value: "support"}}}
	cases := []struct {
		field *models.FieldDef
		in    any
		want  string
	}{
		{dept, "sales", "Sales"},
		{dept, "support", "support"},
		{dept, "gone", "gone"},
		{dept, []any{"sales", "support"}, "Sales; support"},
		{nil, true, "yes"},
		{nil, false, "no"},
		{nil, 2.5, "2.5"},
		{nil, 10.0, "10"},
		{nil, nil, ""},
		{nil, "plain", "plain"},
	}
	for _, c := range cases {
		if got := ExportValue(c.field, c.in); got != c.want {
			t.Errorf("ExportValue(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}
