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
	"gorm.io/gorm/clause"
)

type CampaignMessageRepository struct {
	db *gorm.DB
}

func NewCampaignMessageRepository(db *gorm.DB) *CampaignMessageRepository {
	return &CampaignMessageRepository{db: db}
}

func (r *CampaignMessageRepository) Create(m *models.CampaignMessage) error {
	return r.db.Create(m).Error
}

func (r *CampaignMessageRepository) BulkCreate(messages []models.CampaignMessage) (int, error) {
	if len(messages) == 0 {
		return 0, nil
	}
	result := r.db.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(messages, 100)
	return int(result.RowsAffected), result.Error
}

func (r *CampaignMessageRepository) FindByCampaign(campaignID uint, status string, limit, offset int) ([]models.CampaignMessage, int64, error) {
	var items []models.CampaignMessage
	var total int64

	q := r.db.Model(&models.CampaignMessage{}).Where("campaign_id = ?", campaignID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	q.Count(&total)

	qFind := r.db.Where("campaign_id = ?", campaignID)
	if status != "" {
		qFind = qFind.Where("status = ?", status)
	}
	if err := qFind.Preload("Subscriber").Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *CampaignMessageRepository) FindPendingByCampaign(campaignID uint, batchSize int) ([]models.CampaignMessage, error) {
	var items []models.CampaignMessage
	if err := r.db.Preload("Subscriber").
		Where("campaign_id = ? AND status = ?", campaignID, models.CampaignMsgPending).
		Order("id ASC").
		Limit(batchSize).
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *CampaignMessageRepository) UpdateStatus(id uint, status models.CampaignMessageStatus, errorMsg string) error {
	updates := map[string]interface{}{"status": status}
	if errorMsg != "" {
		updates["error_message"] = errorMsg
	}
	if status == models.CampaignMsgSent {
		now := time.Now()
		updates["sent_at"] = now
	}
	return r.db.Model(&models.CampaignMessage{}).Where("id = ?", id).Updates(updates).Error
}

func (r *CampaignMessageRepository) SetEmailID(id uint, emailID uint) error {
	return r.db.Model(&models.CampaignMessage{}).Where("id = ?", id).Update("email_id", emailID).Error
}

func (r *CampaignMessageRepository) CountByStatus(campaignID uint) (map[models.CampaignMessageStatus]int64, error) {
	type result struct {
		Status models.CampaignMessageStatus
		Count  int64
	}
	var results []result
	if err := r.db.Model(&models.CampaignMessage{}).
		Select("status, COUNT(*) as count").
		Where("campaign_id = ?", campaignID).
		Group("status").
		Find(&results).Error; err != nil {
		return nil, err
	}

	counts := make(map[models.CampaignMessageStatus]int64)
	for _, r := range results {
		counts[r.Status] = r.Count
	}
	return counts, nil
}

func (r *CampaignMessageRepository) CountPending(campaignID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.CampaignMessage{}).
		Where("campaign_id = ? AND status = ?", campaignID, models.CampaignMsgPending).
		Count(&count).Error
	return count, err
}

func (r *CampaignMessageRepository) FindByCampaignMessageID(id uint) (*models.CampaignMessage, error) {
	var msg models.CampaignMessage
	if err := r.db.First(&msg, id).Error; err != nil {
		return nil, err
	}
	return &msg, nil
}

func (r *CampaignMessageRepository) UpdateOpenedAt(id uint) error {
	return r.db.Model(&models.CampaignMessage{}).Where("id = ? AND opened_at IS NULL", id).
		Update("opened_at", time.Now()).Error
}

func (r *CampaignMessageRepository) UpdateClickedAt(id uint) error {
	return r.db.Model(&models.CampaignMessage{}).Where("id = ? AND clicked_at IS NULL", id).
		Update("clicked_at", time.Now()).Error
}

// FindByEmailID looks up a campaign message by its linked email record.
func (r *CampaignMessageRepository) FindByEmailID(emailID uint) (*models.CampaignMessage, error) {
	var msg models.CampaignMessage
	if err := r.db.Where("email_id = ?", emailID).First(&msg).Error; err != nil {
		return nil, err
	}
	return &msg, nil
}

// UpdateBouncedAt marks a campaign message as bounced.
func (r *CampaignMessageRepository) UpdateBouncedAt(id uint) error {
	return r.db.Model(&models.CampaignMessage{}).Where("id = ? AND bounced_at IS NULL", id).
		Update("bounced_at", time.Now()).Error
}

func (r *CampaignMessageRepository) UpdateUnsubscribedAt(id uint) error {
	return r.db.Model(&models.CampaignMessage{}).Where("id = ? AND unsubscribed_at IS NULL", id).
		Update("unsubscribed_at", time.Now()).Error
}
