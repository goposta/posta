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

package widget

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
)

//go:embed posta-form.js
var FormScript []byte

var FormScriptETag = func() string {
	sum := sha256.Sum256(FormScript)
	return `"` + hex.EncodeToString(sum[:12]) + `"`
}()

// ChatScript is the live chat widget, served at /widget/v1/posta-chat.js. It
// is built from posta/widget (make build-widget) and committed, so the server
// builds without Node.
//
//go:embed posta-chat.js
var ChatScript []byte

// ChatScriptETag is a strong ETag over ChatScript.
var ChatScriptETag = func() string {
	sum := sha256.Sum256(ChatScript)
	return `"` + hex.EncodeToString(sum[:12]) + `"`
}()
