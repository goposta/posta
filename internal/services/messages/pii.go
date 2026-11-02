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

// HiddenValue stands in for a personal-data answer in notification emails.
// Members read the real value on the submission page.
const HiddenValue = "•••• (hidden, view in Posta)"

// PIIMask says which of a submission's values a notification email must
// hide: the answers to questions marked personal data, and the summary values
// (sender email, name, phone, subject, body) those questions fill. A form
// without a schema hides nothing.
type PIIMask struct {
	keys  map[string]bool
	roles map[models.FieldRole]bool
}

// NewPIIMask reads the mask from a form schema. Archived questions count:
// older submissions still carry their answers.
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

// Key reports whether the answer submitted under key is hidden.
func (m PIIMask) Key(key string) bool { return m.keys[key] }

// Hides reports whether the question filling role is personal data.
func (m PIIMask) Hides(role models.FieldRole) bool { return m.roles[role] }

// Empty reports whether the form has no personal-data questions.
func (m PIIMask) Empty() bool { return len(m.keys) == 0 }

// Value returns v, or HiddenValue when the question filling role is personal
// data. An empty value stays empty, so nothing is shown as hidden that was
// never given.
func (m PIIMask) Value(role models.FieldRole, v string) string {
	if v == "" || !m.roles[role] {
		return v
	}
	return HiddenValue
}

// Sender labels who submitted, leaving out what is hidden. It returns "" when
// neither a name nor an email was given.
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
