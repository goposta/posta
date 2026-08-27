// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

type SubscriberStatus string

const (
	SubscriberStatusSubscribed   SubscriberStatus = "subscribed"
	SubscriberStatusUnsubscribed SubscriberStatus = "unsubscribed"
	SubscriberStatusBounced      SubscriberStatus = "bounced"
	SubscriberStatusComplained   SubscriberStatus = "complained"
)

// CustomFields is a JSON map stored as TEXT in the database.
type CustomFields map[string]interface{}

func (cf CustomFields) Value() (driver.Value, error) {
	if cf == nil {
		return "{}", nil
	}
	return json.Marshal(cf)
}

func (cf *CustomFields) Scan(value interface{}) error {
	if value == nil {
		*cf = make(CustomFields)
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
	return json.Unmarshal(bytes, cf)
}

type Subscriber struct {
	ID             uint             `json:"id" gorm:"primaryKey"`
	UserID         uint             `json:"user_id" gorm:"index;not null"`
	WorkspaceID    *uint            `json:"workspace_id,omitempty" gorm:"uniqueIndex:idx_sub_scope_email;index"`
	Email          string           `json:"email" gorm:"uniqueIndex:idx_sub_scope_email;not null"`
	Name           string           `json:"name" gorm:"default:''"`
	Status         SubscriberStatus `json:"status" gorm:"type:varchar(20);default:'subscribed';not null;index"`
	CustomFields   CustomFields     `json:"custom_fields" gorm:"type:text"`
	Timezone       string           `json:"timezone" gorm:"size:50;default:''"`
	Language       string           `json:"language" gorm:"size:10;default:''"`
	SubscribedAt   *time.Time       `json:"subscribed_at"`
	UnsubscribedAt *time.Time       `json:"unsubscribed_at"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      *time.Time       `json:"updated_at"`
	User           User             `json:"-" gorm:"foreignKey:UserID"`
}
