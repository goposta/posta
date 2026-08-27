// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

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
