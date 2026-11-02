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
	"testing"

	"github.com/goposta/posta/internal/models"
)

func TestPIIMask(t *testing.T) {
	mask := NewPIIMask(models.FieldDefs{
		{Key: "email", Type: models.FieldEmail, Role: models.RoleSenderEmail, PII: true},
		{Key: "name", Type: models.FieldText, Role: models.RoleSenderName},
		{Key: "ssn", Type: models.FieldText, PII: true, Archived: true},
		{Key: "dept", Type: models.FieldText},
	})

	if !mask.Key("email") || !mask.Key("ssn") || mask.Key("dept") || mask.Key("name") {
		t.Error("Key does not follow the pii flags (archived questions included)")
	}
	if got := mask.Value(models.RoleSenderEmail, "a@b.co"); got != HiddenValue {
		t.Errorf("hidden role value = %q", got)
	}
	if got := mask.Value(models.RoleSenderEmail, ""); got != "" {
		t.Errorf("an empty value became %q", got)
	}
	if got := mask.Value(models.RoleSenderName, "Ada"); got != "Ada" {
		t.Errorf("a visible role value became %q", got)
	}

	cases := []struct{ name, email, want string }{
		{"Ada", "a@b.co", "Ada (email hidden)"},
		{"", "a@b.co", HiddenValue},
		{"Ada", "", "Ada"},
		{"", "", ""},
	}
	for _, c := range cases {
		if got := mask.Sender(c.name, c.email); got != c.want {
			t.Errorf("Sender(%q, %q) = %q, want %q", c.name, c.email, got, c.want)
		}
	}

	none := NewPIIMask(nil)
	if got := none.Sender("Ada", "a@b.co"); got != "Ada <a@b.co>" {
		t.Errorf("schemaless Sender = %q", got)
	}
	both := NewPIIMask(models.FieldDefs{
		{Key: "e", Role: models.RoleSenderEmail, PII: true},
		{Key: "n", Role: models.RoleSenderName, PII: true},
	})
	if got := both.Sender("Ada", "a@b.co"); got != HiddenValue {
		t.Errorf("all-hidden Sender = %q", got)
	}
	if got := both.Sender("", "a@b.co"); got != HiddenValue {
		t.Errorf("hidden email alone = %q", got)
	}
}
