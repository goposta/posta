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
	"fmt"

	"github.com/goposta/posta/internal/models"
)

const HiddenValue = "•••• (hidden, view in Posta)"

// PIIMask also hides the summary values (email, name, phone, subject, body) personal-data questions fill.
type PIIMask struct {
	keys  map[string]bool
	roles map[models.FieldRole]bool
}

// Archived questions count, since older submissions still carry their answers.
func NewPIIMask(defs models.FieldDefs) PIIMask {
	m := PIIMask{keys: map[string]bool{}, roles: map[models.FieldRole]bool{}}
	for _, f := range defs {
		if !f.PII {
			continue
		}
		m.keys[f.Key] = true
		if f.Role != models.RoleNone {
			m.roles[f.Role] = true
		}
	}
	return m
}

func (m PIIMask) Key(key string) bool { return m.keys[key] }

func (m PIIMask) Hides(role models.FieldRole) bool { return m.roles[role] }

func (m PIIMask) Empty() bool { return len(m.keys) == 0 }

// An empty value stays empty, so nothing is shown as hidden that was never given.
func (m PIIMask) Value(role models.FieldRole, v string) string {
	if v == "" || !m.roles[role] {
		return v
	}
	return HiddenValue
}

func (m PIIMask) Sender(name, email string) string {
	nameHidden := name != "" && m.roles[models.RoleSenderName]
	emailHidden := email != "" && m.roles[models.RoleSenderEmail]
	switch {
	case name == "" && email == "":
		return ""
	case (nameHidden || name == "") && (emailHidden || email == ""):
		return HiddenValue
	case emailHidden:
		return name + " (email hidden)"
	case nameHidden || name == "":
		return email
	case email == "":
		return name
	default:
		return fmt.Sprintf("%s <%s>", name, email)
	}
}
