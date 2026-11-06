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
	// Snapshot keeps aggregated stats from soft-delete time so analytics survive CampaignMessage purges.
	Snapshot TemplateData `json:"snapshot,omitempty" gorm:"type:text"`
	User     User         `json:"-" gorm:"foreignKey:UserID"`
}
