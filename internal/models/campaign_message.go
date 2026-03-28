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

import "time"

type CampaignMessageStatus string

const (
	CampaignMsgPending CampaignMessageStatus = "pending"
	CampaignMsgQueued  CampaignMessageStatus = "queued"
	CampaignMsgSent    CampaignMessageStatus = "sent"
	CampaignMsgFailed  CampaignMessageStatus = "failed"
	CampaignMsgSkipped CampaignMessageStatus = "skipped"
)

type CampaignMessage struct {
	ID             uint                  `json:"id" gorm:"primaryKey"`
	CampaignID     uint                  `json:"campaign_id" gorm:"uniqueIndex:idx_campaign_sub;not null;index"`
	SubscriberID   uint                  `json:"subscriber_id" gorm:"uniqueIndex:idx_campaign_sub;not null;index"`
	EmailID        *uint                 `json:"email_id,omitempty" gorm:"index"`
	Status         CampaignMessageStatus `json:"status" gorm:"type:varchar(20);default:'pending';not null;index"`
	ErrorMessage   string                `json:"error_message,omitempty"`
	Variant        string                `json:"variant,omitempty" gorm:"size:50"`
	SentAt         *time.Time            `json:"sent_at,omitempty"`
	OpenedAt       *time.Time            `json:"opened_at,omitempty"`
	ClickedAt      *time.Time            `json:"clicked_at,omitempty"`
	BouncedAt      *time.Time            `json:"bounced_at,omitempty"`
	UnsubscribedAt *time.Time            `json:"unsubscribed_at,omitempty"`
	CreatedAt      time.Time             `json:"created_at"`

	Campaign   Campaign   `json:"-" gorm:"foreignKey:CampaignID"`
	Subscriber Subscriber `json:"-" gorm:"foreignKey:SubscriberID"`
}
