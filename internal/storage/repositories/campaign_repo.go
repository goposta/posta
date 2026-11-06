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

package repositories

import (
	"errors"
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

// FindByIDForScope returns the campaign only if it belongs to scope; prefer it in handlers so scoping lives in one place.
func (r *CampaignRepository) FindByIDForScope(id uint, scope ResourceScope) (*models.Campaign, error) {
	var c models.Campaign
	if err := ApplyScope(r.db, scope).First(&c, id).Error; err != nil {
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

// Delete writes snapshot (nil allowed) before purging the campaign's messages, so post-deletion analytics survive.
func (r *CampaignRepository) Delete(id uint, snapshot models.TemplateData) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if snapshot != nil {
			if err := tx.Model(&models.Campaign{}).Where("id = ?", id).
				Update("snapshot", snapshot).Error; err != nil {
				return err
			}
		}
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

// ErrStatusConflict means no expected source status matched: a concurrent transition or double-click.
var ErrStatusConflict = errors.New("campaign status conflict")

// TransitionStatus atomically moves from one of from to to, returning ErrStatusConflict when no row matched.
func (r *CampaignRepository) TransitionStatus(id uint, from []models.CampaignStatus, to models.CampaignStatus) error {
	updates := map[string]interface{}{"status": to, "updated_at": time.Now()}
	if to == models.CampaignStatusSending {
		updates["started_at"] = time.Now()
	}
	if to == models.CampaignStatusSent || to == models.CampaignStatusCancelled {
		updates["completed_at"] = time.Now()
	}
	res := r.db.Model(&models.Campaign{}).
		Where("id = ? AND status IN ?", id, from).
		Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrStatusConflict
	}
	return nil
}

// FindStuckSending finds campaigns sending longer than stuckFor with pending messages, typically after a worker crash.
func (r *CampaignRepository) FindStuckSending(stuckFor time.Duration) ([]models.Campaign, error) {
	var items []models.Campaign
	cutoff := time.Now().Add(-stuckFor)
	err := r.db.
		Where("status = ? AND (started_at IS NULL OR started_at < ?)", models.CampaignStatusSending, cutoff).
		Find(&items).Error
	return items, err
}

func (r *CampaignRepository) FindScheduledReady() ([]models.Campaign, error) {
	var campaigns []models.Campaign
	if err := r.db.Where("status = ? AND scheduled_at <= ?", models.CampaignStatusScheduled, time.Now()).
		Find(&campaigns).Error; err != nil {
		return nil, err
	}
	return campaigns, nil
}
