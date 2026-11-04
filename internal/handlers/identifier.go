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

// Workspaces, forms, OAuth providers, templates and stylesheets share one
// naming scheme: name is the identifier the API and URLs use, like
// "contact-form"; display_name is the label people read. People create a
// resource by typing only its name, as a label ("Contact form"): the
// identifier is derived from it. A name never changes once the resource
// exists; using another one means creating a new resource.

// legacyNames reads a request made before 0.18, when name was the label and
// slug the identifier: with a slug and no display_name, the slug is the
// identifier and name the label. Deprecated, until 0.19.
func legacyNames(name, displayName, legacySlug string) (string, string) {
	name, displayName, legacySlug = strings.TrimSpace(name), strings.TrimSpace(displayName), strings.TrimSpace(legacySlug)
	if displayName == "" && legacySlug != "" {
		return legacySlug, name
	}
	return name, displayName
}

// resolveNames settles the name and display name of a resource being
// created. A name given alone may be a label, as people type it: it becomes
// the display name and the identifier is derived from it. A missing display
// name is the name. legacySlug is read by legacyNames.
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

// resolveRename settles the display name an update of resource asks for,
// whose name is current; an empty result leaves it as it is. Sending the
// current name is fine, any other is refused. Before 0.18 the label was
// sent as name: a name that is not an identifier, sent without a display
// name, is read as the label (deprecated, until 0.19).
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

// importNames settles the names of a resource read from an export or a
// file, where a missing or malformed identifier must not fail the import:
// it is derived from the display name, or fallback.
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
