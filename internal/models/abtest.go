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
	"database/sql/driver"
	"encoding/json"
)

type ABTestVariant struct {
	Name            string `json:"name"`
	Subject         string `json:"subject"`
	TemplateID      *uint  `json:"template_id,omitempty"`
	SplitPercentage int    `json:"split_percentage"`
}

// ABTestVariants is a JSON array of A/B test variants stored as TEXT.
type ABTestVariants []ABTestVariant

func (v ABTestVariants) Value() (driver.Value, error) {
	if v == nil {
		return "[]", nil
	}
	return json.Marshal(v)
}

func (v *ABTestVariants) Scan(value interface{}) error {
	if value == nil {
		*v = ABTestVariants{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		s, ok := value.(string)
		if !ok {
			return nil
		}
		bytes = []byte(s)
	}
	return json.Unmarshal(bytes, v)
}
