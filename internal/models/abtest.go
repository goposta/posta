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

package models

import (
	"database/sql/driver"
	"encoding/json"
)

// ABTestVariant represents a single variant in an A/B test campaign.
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
