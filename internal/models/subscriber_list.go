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
	"time"
)

type SubscriberListType string

const (
	SubscriberListTypeStatic  SubscriberListType = "static"
	SubscriberListTypeDynamic SubscriberListType = "dynamic"
)

type FilterRule struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    any    `json:"value"`
}

type FilterRules []FilterRule

func (fr FilterRules) Value() (driver.Value, error) {
	if fr == nil {
		return "[]", nil
	}
	return json.Marshal(fr)
}

func (fr *FilterRules) Scan(value interface{}) error {
	if value == nil {
		*fr = FilterRules{}
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
	return json.Unmarshal(bytes, fr)
}

type SubscriberList struct {
	ID          uint               `json:"id" gorm:"primaryKey"`
	UserID      uint               `json:"user_id" gorm:"index;not null"`
	WorkspaceID *uint              `json:"workspace_id,omitempty" gorm:"index"`
	Name        string             `json:"name" gorm:"not null"`
	Description string             `json:"description"`
	Type        SubscriberListType `json:"type" gorm:"type:varchar(10);default:'static';not null"`
	FilterRules FilterRules        `json:"filter_rules,omitempty" gorm:"type:text"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   *time.Time         `json:"updated_at"`
	User        User               `json:"-" gorm:"foreignKey:UserID"`
}

type SubscriberListMember struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	ListID       uint      `json:"list_id" gorm:"uniqueIndex:idx_sublist_sub;not null"`
	SubscriberID uint      `json:"subscriber_id" gorm:"uniqueIndex:idx_sublist_sub;not null;index"`
	CreatedAt    time.Time `json:"created_at"`

	List       SubscriberList `json:"-" gorm:"foreignKey:ListID"`
	Subscriber Subscriber     `json:"-" gorm:"foreignKey:SubscriberID"`
}
