/*
 * Copyright 2026 Jonas Kaninda
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package upgrade

import (
	"regexp"
	"strings"

	"golang.org/x/mod/semver"
)

// gitDescribeSuffix matches the trailing "-N-g<sha>" (plus optional "-dirty")
// that `git describe` appends to commits past a tag. Stripping it lets us
// treat a developer's branch ahead of v1.2.0 as v1.2.0 for downgrade checks
// — not as a pre-release of it.
var gitDescribeSuffix = regexp.MustCompile(`-\d+-g[0-9a-f]+(-dirty)?$`)

// devVersion is the placeholder used by local builds (Makefile fallback).
const devVersion = "dev"

// IsDev reports whether v is the sentinel value used for local/dev builds.
func IsDev(v string) bool { return v == devVersion || v == "" }

// canonical converts a `git describe` style version like "v0.6.6-3-gabc1234"
// or "v0.6.6-3-gabc1234-dirty" into a canonical semver that x/mod/semver can
// compare. Tag-only versions ("v0.6.6") and pre-releases ("v0.6.5-rc.1") pass
// through unchanged.
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

// Compare returns -1, 0, or 1 if a is less than, equal to, or greater than b.
// Invalid or dev versions sort as equal (callers must check IsDev first if
// they care).
func Compare(a, b string) int {
	ca, cb := canonical(a), canonical(b)
	if ca == "" || cb == "" {
		return 0
	}
	return semver.Compare(ca, cb)
}

// IsDowngrade reports whether `binary` is strictly older than `stored`. Both
// arguments are normalized through baseTag so a developer build with commits
// ahead of a tag (e.g. "v1.2.0-3-gabc1234") is treated as the tag itself and
// not as a pre-release of it.
func IsDowngrade(binary, stored string) bool {
	if IsDev(binary) || IsDev(stored) {
		return false
	}
	return Compare(baseTag(binary), baseTag(stored)) < 0
}

// baseTag strips a `git describe` post-tag suffix, leaving the underlying
// release tag. Intentional pre-releases like "v1.0.0-rc.1" pass through.
func baseTag(v string) string {
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	return gitDescribeSuffix.ReplaceAllString(v, "")
}
