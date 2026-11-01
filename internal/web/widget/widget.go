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

// Package widget embeds the scripts that websites load from Posta, served
// under /widget/v1/.
package widget

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
)

// FormScript is the website form widget, served at /widget/v1/posta-form.js.
//
//go:embed posta-form.js
var FormScript []byte

// FormScriptETag is a strong ETag over FormScript, so a cached copy is
// revalidated with a 304 until the script changes.
var FormScriptETag = func() string {
	sum := sha256.Sum256(FormScript)
	return `"` + hex.EncodeToString(sum[:12]) + `"`
}()
