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
	"encoding/json"
	"testing"
)

func TestTemplateRefUnmarshal(t *testing.T) {
	cases := []struct {
		in      string
		want    TemplateRef
		wantErr bool
	}{
		{`{"template":"welcome"}`, "welcome", false},
		{`{"template":"  welcome  "}`, "welcome", false},
		{`{"template":"42"}`, "42", false},
		{`{"template":42}`, "42", false},
		{`{"template":null}`, "", false},
		{`{}`, "", false},
		{`{"template":-1}`, "", true},
		{`{"template":1.5}`, "", true},
		{`{"template":true}`, "", true},
	}
	for _, tc := range cases {
		var req SendTemplateRequest
		err := json.Unmarshal([]byte(tc.in), &req)
		if tc.wantErr {
			if err == nil {
				t.Errorf("%s: expected an error, got template %q", tc.in, req.Template)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", tc.in, err)
			continue
		}
		if req.Template != tc.want {
			t.Errorf("%s: template = %q, want %q", tc.in, req.Template, tc.want)
		}
	}
}

func TestTemplateRefID(t *testing.T) {
	cases := []struct {
		ref    TemplateRef
		wantID uint
		wantOK bool
	}{
		{"42", 42, true},
		{"0", 0, false},
		{"welcome", 0, false},
		{"2024-launch", 0, false},
		{"", 0, false},
	}
	for _, tc := range cases {
		id, ok := tc.ref.ID()
		if id != tc.wantID || ok != tc.wantOK {
			t.Errorf("%q.ID() = %d, %v; want %d, %v", tc.ref, id, ok, tc.wantID, tc.wantOK)
		}
	}
}
