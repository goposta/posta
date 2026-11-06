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
	"fmt"
	"strings"

	"github.com/goposta/posta/internal/models"
)

// name is the immutable identifier ("contact-form"); display_name is the label, and a typed label derives the name.

// legacyNames reads pre-0.18 requests, where name was the label and slug the identifier. Deprecated, until 0.19.
func legacyNames(name, displayName, legacySlug string) (string, string) {
	name, displayName, legacySlug = strings.TrimSpace(name), strings.TrimSpace(displayName), strings.TrimSpace(legacySlug)
	if displayName == "" && legacySlug != "" {
		return legacySlug, name
	}
	return name, displayName
}

// A name given alone may be a label: it becomes the display name and the identifier is derived from it.
func resolveNames(name, displayName, legacySlug string) (string, string, error) {
	name, displayName = legacyNames(name, displayName, legacySlug)
	if displayName == "" && models.CheckIdentifier(name) != nil {
		name, displayName = "", name
	}
	if name == "" {
		name = models.ToIdentifier(displayName)
	}
	if name == "" {
		return "", "", fmt.Errorf("name is required")
	}
	if err := models.CheckIdentifier(name); err != nil {
		return "", "", err
	}
	if displayName == "" {
		displayName = name
	}
	return name, displayName, nil
}

// Only the current name may be sent; a non-identifier name without display_name is a pre-0.18 label (until 0.19).
func resolveRename(name, displayName, legacySlug, current, resource string) (string, error) {
	name, displayName = legacyNames(name, displayName, legacySlug)
	if displayName == "" && name != "" && models.CheckIdentifier(name) != nil {
		name, displayName = "", name
	}
	if name != "" && name != current {
		return "", fmt.Errorf("a %s's name cannot change; create a new %s to use another name", resource, resource)
	}
	return displayName, nil
}

// importNames never fails: a missing or malformed identifier is derived from the display name, or fallback.
func importNames(name, displayName, fallback string) (string, string) {
	name, displayName = legacyNames(name, displayName, "")
	if displayName == "" && models.CheckIdentifier(name) != nil {
		displayName = name
	}
	if name == "" || models.CheckIdentifier(name) != nil {
		name = models.ToIdentifier(firstNonEmpty(name, displayName))
	}
	if name == "" {
		name = fallback
	}
	if displayName == "" {
		displayName = name
	}
	return name, displayName
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
