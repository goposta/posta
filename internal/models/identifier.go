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

package models

import (
	"fmt"
	"regexp"
	"strings"
)

// Shared naming scheme: Name is the API/URL identifier ("contact-form"); DisplayName is the label people read.

var identifierPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

var identifierSeparators = regexp.MustCompile(`[^a-z0-9]+`)

const MaxIdentifierLength = 63

// ToIdentifier turns "Contact form" into "contact-form"; it is empty when the label has no letters or digits.
func ToIdentifier(label string) string {
	v := identifierSeparators.ReplaceAllString(strings.ToLower(strings.TrimSpace(label)), "-")
	v = strings.Trim(v, "-")
	if len(v) > MaxIdentifierLength {
		v = strings.Trim(v[:MaxIdentifierLength], "-")
	}
	return v
}

// CheckIdentifier explains why name cannot be an identifier.
func CheckIdentifier(name string) error {
	if len(name) > MaxIdentifierLength || !identifierPattern.MatchString(name) {
		return fmt.Errorf("name %q must be up to %d lowercase letters, digits and hyphens, like \"contact-form\"; put the label in display_name", name, MaxIdentifierLength)
	}
	return nil
}
