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

	"gorm.io/gorm"
)

type CampaignStatus string

const (
	CampaignStatusDraft     CampaignStatus = "draft"
	CampaignStatusScheduled CampaignStatus = "scheduled"
	CampaignStatusSending   CampaignStatus = "sending"
	CampaignStatusSent      CampaignStatus = "sent"
	CampaignStatusPaused    CampaignStatus = "paused"
	CampaignStatusCancelled CampaignStatus = "cancelled"
)

// TemplateData is a JSON map for template variable substitution.
type TemplateData map[string]interface{}

func (td TemplateData) Value() (driver.Value, error) {
	if td == nil {
		return "{}", nil
	}
	return json.Marshal(td)
}

func (td *TemplateData) Scan(value interface{}) error {
	if value == nil {
		*td = make(TemplateData)
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
	return json.Unmarshal(bytes, td)
}

type Campaign struct {
	ID                uint           `json:"id" gorm:"primaryKey"`
	UserID            uint           `json:"user_id" gorm:"index;not null"`
	WorkspaceID       *uint          `json:"workspace_id,omitempty" gorm:"index"`
	Name              string         `json:"name" gorm:"not null"`
	Subject           string         `json:"subject" gorm:"not null"`
	FromEmail         string         `json:"from_email" gorm:"not null"`
	FromName          string         `json:"from_name"`
	TemplateID        uint           `json:"template_id" gorm:"index;not null"`
	TemplateVersionID *uint          `json:"template_version_id,omitempty" gorm:"index"`
	Language          string         `json:"language" gorm:"default:'en'"`
	TemplateData      TemplateData   `json:"template_data,omitempty" gorm:"type:text"`
	Status            CampaignStatus `json:"status" gorm:"type:varchar(20);default:'draft';not null;index"`
	ListID            uint           `json:"list_id" gorm:"index;not null"`
	SendRate          int            `json:"send_rate" gorm:"default:0"`
	SendAtLocalTime   bool           `json:"send_at_local_time" gorm:"default:false"`
	ABTestEnabled     bool           `json:"ab_test_enabled" gorm:"default:false"`
	ABTestVariants    ABTestVariants `json:"ab_test_variants,omitempty" gorm:"type:text"`
	ABTestWinner      string         `json:"ab_test_winner,omitempty"`
	ScheduledAt       *time.Time     `json:"scheduled_at,omitempty"`
	StartedAt         *time.Time     `json:"started_at,omitempty"`
	CompletedAt       *time.Time     `json:"completed_at,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         *time.Time     `json:"updated_at,omitempty"`
	DeletedAt         gorm.DeletedAt `json:"-" gorm:"index"`
	// Snapshot captures the aggregated stats at soft-delete time so analytics
	// for past sends survive after CampaignMessage rows are purged.
	Snapshot TemplateData `json:"snapshot,omitempty" gorm:"type:text"`
	User     User         `json:"-" gorm:"foreignKey:UserID"`
}
