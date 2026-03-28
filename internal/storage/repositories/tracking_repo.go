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

package repositories

import (
	"time"

	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
)

type TrackingRepository struct {
	db *gorm.DB
}

func NewTrackingRepository(db *gorm.DB) *TrackingRepository {
	return &TrackingRepository{db: db}
}


func (r *TrackingRepository) CreateLink(link *models.TrackedLink) error {
	return r.db.Create(link).Error
}

func (r *TrackingRepository) FindLinkByHash(hash string) (*models.TrackedLink, error) {
	var link models.TrackedLink
	if err := r.db.Where("hash = ?", hash).First(&link).Error; err != nil {
		return nil, err
	}
	return &link, nil
}

func (r *TrackingRepository) FindLinksByCampaign(campaignID uint) ([]models.TrackedLink, error) {
	var links []models.TrackedLink
	if err := r.db.Where("campaign_id = ?", campaignID).
		Order("click_count DESC").Find(&links).Error; err != nil {
		return nil, err
	}
	return links, nil
}

func (r *TrackingRepository) IncrementLinkClickCount(linkID uint) {
	r.db.Model(&models.TrackedLink{}).Where("id = ?", linkID).
		Update("click_count", gorm.Expr("click_count + 1"))
}

// FindOrCreateLink finds existing or creates a new tracked link.
func (r *TrackingRepository) FindOrCreateLink(campaignID uint, originalURL, hash string) (*models.TrackedLink, error) {
	var link models.TrackedLink
	if err := r.db.Where("hash = ?", hash).First(&link).Error; err == nil {
		return &link, nil
	}
	link = models.TrackedLink{
		CampaignID:  campaignID,
		OriginalURL: originalURL,
		Hash:        hash,
	}
	if err := r.db.Create(&link).Error; err != nil {
		// Race condition: try to find again
		if err2 := r.db.Where("hash = ?", hash).First(&link).Error; err2 == nil {
			return &link, nil
		}
		return nil, err
	}
	return &link, nil
}


func (r *TrackingRepository) CreateEvent(event *models.TrackingEvent) error {
	return r.db.Create(event).Error
}

// HasEvent checks if an event of this type already exists for a campaign message (deduplication).
func (r *TrackingRepository) HasEvent(campaignMessageID uint, eventType models.TrackingEventType) bool {
	var count int64
	r.db.Model(&models.TrackingEvent{}).
		Where("campaign_message_id = ? AND event_type = ?", campaignMessageID, eventType).
		Count(&count)
	return count > 0
}

// HasClickEvent checks if a click event already exists for a specific link.
func (r *TrackingRepository) HasClickEvent(campaignMessageID uint, trackedLinkID uint) bool {
	var count int64
	r.db.Model(&models.TrackingEvent{}).
		Where("campaign_message_id = ? AND event_type = ? AND tracked_link_id = ?",
			campaignMessageID, models.TrackingEventClick, trackedLinkID).
		Count(&count)
	return count > 0
}


type CampaignAnalytics struct {
	TotalMessages    int64   `json:"total_messages"`
	SentMessages     int64   `json:"sent_messages"`
	FailedMessages   int64   `json:"failed_messages"`
	OpenedMessages   int64   `json:"opened_messages"`
	ClickedMessages  int64   `json:"clicked_messages"`
	BouncedMessages  int64   `json:"bounced_messages"`
	Unsubscribed     int64   `json:"unsubscribed"`
	DeliveryRate     float64 `json:"delivery_rate"`
	OpenRate         float64 `json:"open_rate"`
	ClickRate        float64 `json:"click_rate"`
	BounceRate       float64 `json:"bounce_rate"`
	UnsubscribeRate  float64 `json:"unsubscribe_rate"`
}

func (r *TrackingRepository) CampaignAnalytics(campaignID uint) (*CampaignAnalytics, error) {
	a := &CampaignAnalytics{}

	// Single aggregation query instead of 7 separate COUNT queries
	err := r.db.Model(&models.CampaignMessage{}).
		Select(`COUNT(*) as total_messages,
			COUNT(CASE WHEN status = ? THEN 1 END) as sent_messages,
			COUNT(CASE WHEN status = ? THEN 1 END) as failed_messages,
			COUNT(CASE WHEN opened_at IS NOT NULL THEN 1 END) as opened_messages,
			COUNT(CASE WHEN clicked_at IS NOT NULL THEN 1 END) as clicked_messages,
			COUNT(CASE WHEN bounced_at IS NOT NULL THEN 1 END) as bounced_messages,
			COUNT(CASE WHEN unsubscribed_at IS NOT NULL THEN 1 END) as unsubscribed`,
			models.CampaignMsgSent, models.CampaignMsgFailed).
		Where("campaign_id = ?", campaignID).
		Scan(a).Error
	if err != nil {
		return nil, err
	}

	if a.TotalMessages > 0 {
		a.DeliveryRate = float64(a.SentMessages) / float64(a.TotalMessages) * 100
		a.BounceRate = float64(a.BouncedMessages) / float64(a.TotalMessages) * 100
		a.UnsubscribeRate = float64(a.Unsubscribed) / float64(a.TotalMessages) * 100
	}
	if a.SentMessages > 0 {
		a.OpenRate = float64(a.OpenedMessages) / float64(a.SentMessages) * 100
		a.ClickRate = float64(a.ClickedMessages) / float64(a.SentMessages) * 100
	}

	return a, nil
}

type TimeSeriesPoint struct {
	Time  string `json:"time"`
	Count int64  `json:"count"`
}

// EventTimeSeries returns hourly event counts since campaign started.
func (r *TrackingRepository) EventTimeSeries(campaignID uint, eventType models.TrackingEventType) ([]TimeSeriesPoint, error) {
	var results []TimeSeriesPoint
	err := r.db.Table("tracking_events").
		Select("TO_CHAR(tracking_events.created_at, 'YYYY-MM-DD HH24:00') as time, COUNT(*) as count").
		Joins("JOIN campaign_messages ON campaign_messages.id = tracking_events.campaign_message_id").
		Where("campaign_messages.campaign_id = ? AND tracking_events.event_type = ?", campaignID, eventType).
		Group("time").Order("time ASC").
		Find(&results).Error
	return results, err
}

// CampaignAnalyticsByVariant returns analytics grouped by A/B test variant.
func (r *TrackingRepository) CampaignAnalyticsByVariant(campaignID uint) (map[string]*CampaignAnalytics, error) {
	type variantRow struct {
		Variant          string
		TotalMessages    int64
		SentMessages     int64
		FailedMessages   int64
		OpenedMessages   int64
		ClickedMessages  int64
		BouncedMessages  int64
		Unsubscribed     int64
	}
	var rows []variantRow
	err := r.db.Model(&models.CampaignMessage{}).
		Select(`variant,
			COUNT(*) as total_messages,
			COUNT(CASE WHEN status = ? THEN 1 END) as sent_messages,
			COUNT(CASE WHEN status = ? THEN 1 END) as failed_messages,
			COUNT(CASE WHEN opened_at IS NOT NULL THEN 1 END) as opened_messages,
			COUNT(CASE WHEN clicked_at IS NOT NULL THEN 1 END) as clicked_messages,
			COUNT(CASE WHEN bounced_at IS NOT NULL THEN 1 END) as bounced_messages,
			COUNT(CASE WHEN unsubscribed_at IS NOT NULL THEN 1 END) as unsubscribed`,
			models.CampaignMsgSent, models.CampaignMsgFailed).
		Where("campaign_id = ? AND variant != ''", campaignID).
		Group("variant").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	result := make(map[string]*CampaignAnalytics)
	for _, row := range rows {
		a := &CampaignAnalytics{
			TotalMessages:   row.TotalMessages,
			SentMessages:    row.SentMessages,
			FailedMessages:  row.FailedMessages,
			OpenedMessages:  row.OpenedMessages,
			ClickedMessages: row.ClickedMessages,
			BouncedMessages: row.BouncedMessages,
			Unsubscribed:    row.Unsubscribed,
		}
		if a.TotalMessages > 0 {
			a.DeliveryRate = float64(a.SentMessages) / float64(a.TotalMessages) * 100
			a.BounceRate = float64(a.BouncedMessages) / float64(a.TotalMessages) * 100
			a.UnsubscribeRate = float64(a.Unsubscribed) / float64(a.TotalMessages) * 100
		}
		if a.SentMessages > 0 {
			a.OpenRate = float64(a.OpenedMessages) / float64(a.SentMessages) * 100
			a.ClickRate = float64(a.ClickedMessages) / float64(a.SentMessages) * 100
		}
		result[row.Variant] = a
	}
	return result, nil
}

// DeleteOlderThan deletes tracking events older than the given time.
func (r *TrackingRepository) DeleteOlderThan(before time.Time) (int64, error) {
	result := r.db.Where("created_at < ?", before).Delete(&models.TrackingEvent{})
	return result.RowsAffected, result.Error
}
