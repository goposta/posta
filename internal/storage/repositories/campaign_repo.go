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

// FindByIDForScope returns the campaign only if it belongs to the given scope.
// Prefer this over FindByID in request handlers so scoping lives in one place.
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

// Delete soft-deletes the campaign and purges its CampaignMessage rows.
// A snapshot of aggregated stats is written to campaigns.snapshot first so
// post-deletion analytics survive. The snapshot is optional; callers pass nil
// for a bare delete (e.g., never-sent drafts).
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

// ErrStatusConflict indicates the campaign was not in any of the expected source statuses,
// so the transition did not happen. Used to detect concurrent writes / double-clicks.
var ErrStatusConflict = errors.New("campaign status conflict")

// TransitionStatus atomically moves a campaign from one of `from` statuses to `to`.
// Returns ErrStatusConflict when no row matched (concurrent transition).
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

// FindStuckSending returns campaigns that have been in `sending` status longer
// than `stuckFor` but still have pending messages — typically the result of a
// worker crash between enqueue and completion. Used by the restart sweep.
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
