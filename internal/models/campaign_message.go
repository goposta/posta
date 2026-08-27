// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

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
