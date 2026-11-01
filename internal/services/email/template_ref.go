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

package email

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// TemplateRef names a template by name or by ID. It decodes from a JSON
// string ("welcome", "42") or a JSON number (42).
type TemplateRef string

func (r *TemplateRef) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) {
		*r = ""
		return nil
	}
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*r = TemplateRef(strings.TrimSpace(s))
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(data, &n); err != nil {
		return fmt.Errorf("template must be a name or an ID")
	}
	if _, err := strconv.ParseUint(n.String(), 10, 64); err != nil {
		return fmt.Errorf("template ID must be a positive integer")
	}
	*r = TemplateRef(n.String())
	return nil
}

func (r TemplateRef) String() string { return string(r) }

// ID returns the template ID r denotes when it is all digits.
func (r TemplateRef) ID() (uint, bool) {
	id, err := strconv.ParseUint(string(r), 10, 0)
	if err != nil || id == 0 {
		return 0, false
	}
	return uint(id), true
}
