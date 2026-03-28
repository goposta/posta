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

type CampaignRepository struct {
	db *gorm.DB
}

func NewCampaignRepository(db *gorm.DB) *CampaignRepository {
	return &CampaignRepository{db: db}
}

func (r *CampaignRepository) Create(c *models.Campaign) error {
	return r.db.Create(c).Error
}

func (r *CampaignRepository) FindByID(id uint) (*models.Campaign, error) {
	var c models.Campaign
	if err := r.db.First(&c, id).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *CampaignRepository) FindByScope(scope ResourceScope, status string, limit, offset int) ([]models.Campaign, int64, error) {
	var items []models.Campaign
	var total int64

	q := ApplyScope(r.db.Model(&models.Campaign{}), scope)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	q.Count(&total)

	qFind := ApplyScope(r.db, scope)
	if status != "" {
		qFind = qFind.Where("status = ?", status)
	}
	if err := qFind.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *CampaignRepository) Update(c *models.Campaign) error {
	return r.db.Save(c).Error
}

func (r *CampaignRepository) Delete(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("campaign_id = ?", id).Delete(&models.CampaignMessage{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.Campaign{}, id).Error
	})
}

func (r *CampaignRepository) UpdateStatus(id uint, status models.CampaignStatus) error {
	updates := map[string]interface{}{"status": status, "updated_at": time.Now()}
	if status == models.CampaignStatusSending {
		updates["started_at"] = time.Now()
	}
	if status == models.CampaignStatusSent || status == models.CampaignStatusCancelled {
		updates["completed_at"] = time.Now()
	}
	return r.db.Model(&models.Campaign{}).Where("id = ?", id).Updates(updates).Error
}

func (r *CampaignRepository) FindScheduledReady() ([]models.Campaign, error) {
	var campaigns []models.Campaign
	if err := r.db.Where("status = ? AND scheduled_at <= ?", models.CampaignStatusScheduled, time.Now()).
		Find(&campaigns).Error; err != nil {
		return nil, err
	}
	return campaigns, nil
}
