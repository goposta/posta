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

package upgrade

import (
	"regexp"
	"strings"

	"golang.org/x/mod/semver"
)

// gitDescribeSuffix lets a branch ahead of v1.2.0 count as v1.2.0, not a pre-release of it, in downgrade checks.
var gitDescribeSuffix = regexp.MustCompile(`-\d+-g[0-9a-f]+(-dirty)?$`)

// devVersion is the placeholder used by local builds (Makefile fallback).
const devVersion = "dev"

func IsDev(v string) bool { return v == devVersion || v == "" }

// canonical turns `git describe` versions like v0.6.6-3-gabc1234 into semver; tags and pre-releases pass through.
func canonical(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	if c := semver.Canonical(v); c != "" {
		return c
	}
	// Fall back: strip everything after the first '-' (post-tag commit suffix).
	if idx := strings.Index(v, "-"); idx > 0 {
		return semver.Canonical(v[:idx])
	}
	return ""
}

// Compare treats invalid or dev versions as equal; callers must check IsDev first if they care.
func Compare(a, b string) int {
	ca, cb := canonical(a), canonical(b)
	if ca == "" || cb == "" {
		return 0
	}
	return semver.Compare(ca, cb)
}

// IsDowngrade treats a build with commits ahead of a tag (v1.2.0-3-gabc1234) as the tag, not a pre-release of it.
func IsDowngrade(binary, stored string) bool {
	if IsDev(binary) || IsDev(stored) {
		return false
	}
	return Compare(baseTag(binary), baseTag(stored)) < 0
}

// baseTag strips a `git describe` post-tag suffix; intentional pre-releases like v1.0.0-rc.1 pass through.
func baseTag(v string) string {
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	return gitDescribeSuffix.ReplaceAllString(v, "")
}
