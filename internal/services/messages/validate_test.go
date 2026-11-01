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
	"strings"
	"testing"

	"github.com/goposta/posta/internal/models"
)

func ptrF(v float64) *float64 { return &v }
func ptrI(v int) *int         { return &v }

func fieldsOf(kv ...string) []models.MessageField {
	out := make([]models.MessageField, 0, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		out = append(out, models.MessageField{Key: kv[i], Value: kv[i+1]})
	}
	return out
}

func TestValidateAcceptsAndCoerces(t *testing.T) {
	form := &models.Form{Fields: models.FieldDefs{
		{Key: "email", Label: "Email", Type: models.FieldEmail, Required: true, Role: models.RoleSenderEmail},
		{Key: "age", Label: "Age", Type: models.FieldNumber, Min: ptrF(18), Max: ptrF(120)},
		{Key: "dept", Label: "Department", Type: models.FieldSelect, Options: []models.FieldOption{{Value: "sales", Label: "Sales"}, {Value: "support"}}},
		{Key: "topics", Label: "Topics", Type: models.FieldMultiSelect, Options: []models.FieldOption{{Value: "a, b"}, {Value: "c"}}},
		{Key: "terms", Label: "Terms", Type: models.FieldCheckbox, Required: true},
		{Key: "news", Label: "Newsletter", Type: models.FieldCheckbox},
		{Key: "score", Label: "Score", Type: models.FieldScale, Min: ptrF(0), Max: ptrF(10)},
		{Key: "stars", Label: "Stars", Type: models.FieldRating},
		{Key: "day", Label: "Day", Type: models.FieldDate},
		{Key: "at", Label: "At", Type: models.FieldTime},
		{Key: "site", Label: "Site", Type: models.FieldURL},
		{Key: "phone", Label: "Phone", Type: models.FieldPhone},
		{Key: "note", Label: "Note", Type: models.FieldTextarea, Role: models.RoleBody},
	}}
	got, errs := Validate(form, fieldsOf(
		"email", "Ada <ada@example.com>", "age", "36", "dept", "sales",
		"topics", "a, b", "topics", "c", "topics", "c",
		"terms", "on", "score", "0", "stars", "5", "day", "2026-11-01", "at", "09:30:00",
		"site", "https://example.com/x", "phone", "+44 (20) 7946-0958", "note", "Hello",
		"extra", "kept",
	))
	if errs != nil {
		t.Fatalf("unexpected errors: %+v", errs)
	}
	want := models.SubmissionData{
		"email": "ada@example.com", "age": float64(36), "dept": "sales",
		"topics": []string{"a, b", "c"}, "terms": true, "news": false,
		"score": 0, "stars": 5, "day": "2026-11-01", "at": "09:30",
		"site": "https://example.com/x", "phone": "+44 (20) 7946-0958", "note": "Hello",
	}
	if !reflect.DeepEqual(got.Data, want) {
		t.Errorf("data:\n got %#v\nwant %#v", got.Data, want)
	}
	display := map[string]string{}
	keys := []string{}
	for _, f := range got.Fields {
		display[f.Key] = f.Value
		keys = append(keys, f.Key)
	}
	if display["dept"] != "Sales" || display["terms"] != "Yes" || display["news"] != "No" || display["topics"] != "a, b, c" {
		t.Errorf("display copy: %v", display)
	}
	if keys[0] != "email" || keys[len(keys)-1] != "extra" {
		t.Errorf("display order should follow the schema, then unknown fields: %v", keys)
	}
	if got.Roles[models.RoleSenderEmail] != "ada@example.com" || got.Roles[models.RoleBody] != "Hello" {
		t.Errorf("roles: %v", got.Roles)
	}
}

func TestValidateRejects(t *testing.T) {
	cases := []struct {
		name  string
		def   models.FieldDef
		value []string // nil: field absent
		code  string
	}{
		{"required text", models.FieldDef{Type: models.FieldText, Required: true}, nil, "required"},
		{"blank required", models.FieldDef{Type: models.FieldText, Required: true}, []string{"   "}, "required"},
		{"too short", models.FieldDef{Type: models.FieldText, MinLen: ptrI(3)}, []string{"ab"}, "too_short"},
		{"default text limit", models.FieldDef{Type: models.FieldText}, []string{strings.Repeat("x", 201)}, "too_long"},
		{"pattern is a full match", models.FieldDef{Type: models.FieldText, Pattern: `[A-Z]{3}`}, []string{"ABCD"}, "pattern"},
		{"email", models.FieldDef{Type: models.FieldEmail}, []string{"not-an-email"}, "invalid_email"},
		{"email without a dot", models.FieldDef{Type: models.FieldEmail}, []string{"a@localhost"}, "invalid_email"},
		{"phone", models.FieldDef{Type: models.FieldPhone}, []string{"12-34"}, "invalid_phone"},
		{"number", models.FieldDef{Type: models.FieldNumber}, []string{"twelve"}, "invalid_number"},
		{"NaN", models.FieldDef{Type: models.FieldNumber}, []string{"NaN"}, "invalid_number"},
		{"number range", models.FieldDef{Type: models.FieldNumber, Max: ptrF(10)}, []string{"11"}, "out_of_range"},
		{"url scheme", models.FieldDef{Type: models.FieldURL}, []string{"javascript:alert(1)"}, "invalid_url"},
		{"date", models.FieldDef{Type: models.FieldDate}, []string{"01/11/2026"}, "invalid_date"},
		{"date range", models.FieldDef{Type: models.FieldDate}, []string{"1800-01-01"}, "invalid_date"},
		{"time", models.FieldDef{Type: models.FieldTime}, []string{"25:00"}, "invalid_time"},
		{"select", models.FieldDef{Type: models.FieldSelect, Options: []models.FieldOption{{Value: "a"}}}, []string{"b"}, "invalid_option"},
		{"multiselect", models.FieldDef{Type: models.FieldMultiSelect, Options: []models.FieldOption{{Value: "a"}}}, []string{"a", "b"}, "invalid_option"},
		{"required checkbox must be ticked", models.FieldDef{Type: models.FieldCheckbox, Required: true}, []string{"false"}, "required"},
		{"checkbox value", models.FieldDef{Type: models.FieldCheckbox}, []string{"maybe"}, "invalid_value"},
		{"scale range", models.FieldDef{Type: models.FieldScale}, []string{"6"}, "out_of_range"},
		{"rating range", models.FieldDef{Type: models.FieldRating}, []string{"0"}, "out_of_range"},
	}
	for _, tc := range cases {
		tc.def.Key, tc.def.Label = "f", "F"
		form := &models.Form{Fields: models.FieldDefs{tc.def}}
		var fields []models.MessageField
		for _, v := range tc.value {
			fields = append(fields, models.MessageField{Key: "f", Value: v})
		}
		_, errs := Validate(form, fields)
		if len(errs) != 1 || errs[0].Code != tc.code || errs[0].Key != "f" || errs[0].Message == "" {
			t.Errorf("%s: errors = %+v, want one %q", tc.name, errs, tc.code)
		}
	}
}

func TestValidateUnknownFields(t *testing.T) {
	defs := models.FieldDefs{
		{Key: "email", Label: "Email", Type: models.FieldEmail},
		{Key: "old", Label: "Old", Type: models.FieldText, Archived: true},
	}
	submitted := fieldsOf("email", "a@example.com", "old", "legacy", "extra", "x")
	keys := func(fs []models.MessageField) string {
		var k []string
		for _, f := range fs {
			k = append(k, f.Key)
		}
		return strings.Join(k, ",")
	}

	keep, errs := Validate(&models.Form{Fields: defs}, submitted)
	if errs != nil || keys(keep.Fields) != "email,old,extra" {
		t.Errorf("keep: fields %q, errors %+v", keys(keep.Fields), errs)
	}
	if _, inData := keep.Data["extra"]; inData {
		t.Error("keep: an unknown field must not enter typed data")
	}

	drop, errs := Validate(&models.Form{Fields: defs, UnknownFields: models.UnknownFieldsDrop}, submitted)
	if errs != nil || keys(drop.Fields) != "email" {
		t.Errorf("drop: fields %q, errors %+v", keys(drop.Fields), errs)
	}

	_, errs = Validate(&models.Form{Fields: defs, UnknownFields: models.UnknownFieldsReject}, submitted)
	if len(errs) != 1 || errs[0].Key != "extra" || errs[0].Code != "unknown_field" {
		t.Errorf("reject: errors %+v; archived keys must not be rejected", errs)
	}
}

func TestExtractWithRolesFallsBackOnlyForUnsetRoles(t *testing.T) {
	schema := models.FieldDefs{
		{Key: "contact", Label: "Contact", Type: models.FieldEmail, Role: models.RoleSenderEmail},
		{Key: "question", Label: "Question", Type: models.FieldTextarea, Role: models.RoleBody},
	}
	fields := fieldsOf("contact", "c@example.com", "email", "decoy@example.com", "name", "Ada", "question", "Hi")
	got := ExtractWithRoles(map[models.FieldRole]string{
		models.RoleSenderEmail: "c@example.com", models.RoleBody: "Hi",
	}, schema, fields)
	if got.Email != "c@example.com" || got.Body != "Hi" || got.Name != "Ada" {
		t.Errorf("got %+v: roles must win, unset roles fall back to guessing", got)
	}
}

func TestCheckSchema(t *testing.T) {
	ok := models.FieldDefs{{Key: "email", Label: "Email", Type: models.FieldEmail, Role: models.RoleSenderEmail}}
	if err := CheckSchema(ok, nil, "_gotcha"); err != nil {
		t.Fatalf("valid schema rejected: %v", err)
	}
	cases := []struct {
		name string
		defs models.FieldDefs
		prev models.FieldDefs
		want string
	}{
		{"bad key", models.FieldDefs{{Key: "Bad Key", Label: "x", Type: models.FieldText}}, nil, "lowercase"},
		{"reserved key", models.FieldDefs{{Key: "_nonce", Label: "x", Type: models.FieldText}}, nil, "lowercase"},
		{"duplicate key", models.FieldDefs{{Key: "a", Label: "a", Type: models.FieldText}, {Key: "a", Label: "a", Type: models.FieldText}}, nil, "twice"},
		{"honeypot key", models.FieldDefs{{Key: "website", Label: "x", Type: models.FieldText}}, nil, "honeypot"},
		{"unknown type", models.FieldDefs{{Key: "a", Label: "a", Type: "password"}}, nil, "unknown type"},
		{"missing label", models.FieldDefs{{Key: "a", Type: models.FieldText}}, nil, "label"},
		{"role on wrong type", models.FieldDefs{{Key: "a", Label: "a", Type: models.FieldText, Role: models.RoleSenderEmail}}, nil, "does not fit"},
		{"role twice", models.FieldDefs{{Key: "a", Label: "a", Type: models.FieldEmail, Role: models.RoleSenderEmail}, {Key: "b", Label: "b", Type: models.FieldEmail, Role: models.RoleSenderEmail}}, nil, "both have role"},
		{"bad pattern", models.FieldDefs{{Key: "a", Label: "a", Type: models.FieldText, Pattern: "("}}, nil, "does not compile"},
		{"select without options", models.FieldDefs{{Key: "a", Label: "a", Type: models.FieldSelect}}, nil, "options"},
		{"duplicate option", models.FieldDefs{{Key: "a", Label: "a", Type: models.FieldSelect, Options: []models.FieldOption{{Value: "x"}, {Value: "x"}}}}, nil, "twice"},
		{"display on wrong type", models.FieldDefs{{Key: "a", Label: "a", Type: models.FieldSelect, Display: models.DisplayCheckboxes, Options: []models.FieldOption{{Value: "x"}}}}, nil, "display"},
		{"scale range", models.FieldDefs{{Key: "a", Label: "a", Type: models.FieldScale, Min: ptrF(2), Max: ptrF(5)}}, nil, "scale"},
		{"rating range", models.FieldDefs{{Key: "a", Label: "a", Type: models.FieldRating, Max: ptrF(20)}}, nil, "stars"},
		{"removed key", models.FieldDefs{}, ok, "archive it"},
	}
	for _, tc := range cases {
		err := CheckSchema(tc.defs, tc.prev, "website")
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want it to mention %q", tc.name, err, tc.want)
		}
	}
	archived := models.FieldDefs{{Key: "email", Label: "Email", Type: models.FieldEmail, Archived: true}}
	if err := CheckSchema(archived, ok, "_gotcha"); err != nil {
		t.Errorf("archiving a key must be allowed: %v", err)
	}
}

func TestJoinRepeated(t *testing.T) {
	got := JoinRepeated(fieldsOf("a", "1", "b", "x", "a", "2", "a", ""))
	want := fieldsOf("a", "1, 2", "b", "x")
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
