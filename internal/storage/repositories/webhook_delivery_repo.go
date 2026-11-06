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
	"time"

	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
)

type WebhookDeliveryRepository struct {
	db *gorm.DB
}

func NewWebhookDeliveryRepository(db *gorm.DB) *WebhookDeliveryRepository {
	return &WebhookDeliveryRepository{db: db}
}

func (r *WebhookDeliveryRepository) Create(d *models.WebhookDelivery) error {
	return r.db.Create(d).Error
}

type WebhookDeliveryStats struct {
	TotalDeliveries   int64   `json:"total_deliveries"`
	SuccessDeliveries int64   `json:"success_deliveries"`
	FailedDeliveries  int64   `json:"failed_deliveries"`
	SuccessRate       float64 `json:"success_rate"`
}

func (r *WebhookDeliveryRepository) StatsByUserID(userID uint) (*WebhookDeliveryStats, error) {
	var stats WebhookDeliveryStats

	r.db.Model(&models.WebhookDelivery{}).Where("user_id = ? AND workspace_id IS NULL", userID).Count(&stats.TotalDeliveries)
	r.db.Model(&models.WebhookDelivery{}).Where("user_id = ? AND workspace_id IS NULL AND status = ?", userID, models.WebhookDeliverySuccess).Count(&stats.SuccessDeliveries)
	r.db.Model(&models.WebhookDelivery{}).Where("user_id = ? AND workspace_id IS NULL AND status = ?", userID, models.WebhookDeliveryFailed).Count(&stats.FailedDeliveries)

	if stats.TotalDeliveries > 0 {
		stats.SuccessRate = float64(stats.SuccessDeliveries) / float64(stats.TotalDeliveries) * 100
	}

	return &stats, nil
}

func (r *WebhookDeliveryRepository) StatsByWorkspaceID(workspaceID uint) (*WebhookDeliveryStats, error) {
	var stats WebhookDeliveryStats

	r.db.Model(&models.WebhookDelivery{}).Where("workspace_id = ?", workspaceID).Count(&stats.TotalDeliveries)
	r.db.Model(&models.WebhookDelivery{}).Where("workspace_id = ? AND status = ?", workspaceID, models.WebhookDeliverySuccess).Count(&stats.SuccessDeliveries)
	r.db.Model(&models.WebhookDelivery{}).Where("workspace_id = ? AND status = ?", workspaceID, models.WebhookDeliveryFailed).Count(&stats.FailedDeliveries)

	if stats.TotalDeliveries > 0 {
		stats.SuccessRate = float64(stats.SuccessDeliveries) / float64(stats.TotalDeliveries) * 100
	}

	return &stats, nil
}

func (r *WebhookDeliveryRepository) StatsAll() (*WebhookDeliveryStats, error) {
	var stats WebhookDeliveryStats

	r.db.Model(&models.WebhookDelivery{}).Count(&stats.TotalDeliveries)
	r.db.Model(&models.WebhookDelivery{}).Where("status = ?", models.WebhookDeliverySuccess).Count(&stats.SuccessDeliveries)
	r.db.Model(&models.WebhookDelivery{}).Where("status = ?", models.WebhookDeliveryFailed).Count(&stats.FailedDeliveries)

	if stats.TotalDeliveries > 0 {
		stats.SuccessRate = float64(stats.SuccessDeliveries) / float64(stats.TotalDeliveries) * 100
	}

	return &stats, nil
}

func (r *WebhookDeliveryRepository) RecentByUserID(userID uint, limit int) ([]models.WebhookDelivery, error) {
	var deliveries []models.WebhookDelivery
	err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Find(&deliveries).Error
	return deliveries, err
}

func (r *WebhookDeliveryRepository) FindByUserID(userID uint, limit, offset int) ([]models.WebhookDelivery, int64, error) {
	var deliveries []models.WebhookDelivery
	var total int64
	r.db.Model(&models.WebhookDelivery{}).Where("user_id = ? AND workspace_id IS NULL", userID).Count(&total)
	err := r.db.Where("user_id = ? AND workspace_id IS NULL", userID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&deliveries).Error
	return deliveries, total, err
}

func (r *WebhookDeliveryRepository) FindByWorkspaceID(workspaceID uint, limit, offset int) ([]models.WebhookDelivery, int64, error) {
	var deliveries []models.WebhookDelivery
	var total int64
	r.db.Model(&models.WebhookDelivery{}).Where("workspace_id = ?", workspaceID).Count(&total)
	err := r.db.Where("workspace_id = ?", workspaceID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&deliveries).Error
	return deliveries, total, err
}

func (r *WebhookDeliveryRepository) FindByScope(scope ResourceScope, limit, offset int) ([]models.WebhookDelivery, int64, error) {
	var items []models.WebhookDelivery
	var total int64

	ApplyScope(r.db.Model(&models.WebhookDelivery{}), scope).Count(&total)

	if err := ApplyScope(r.db, scope).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *WebhookDeliveryRepository) StatsByScope(scope ResourceScope) (*WebhookDeliveryStats, error) {
	var stats WebhookDeliveryStats

	ApplyScope(r.db.Model(&models.WebhookDelivery{}), scope).Count(&stats.TotalDeliveries)
	ApplyScope(r.db.Model(&models.WebhookDelivery{}), scope).Where("status = ?", models.WebhookDeliverySuccess).Count(&stats.SuccessDeliveries)
	ApplyScope(r.db.Model(&models.WebhookDelivery{}), scope).Where("status = ?", models.WebhookDeliveryFailed).Count(&stats.FailedDeliveries)

	if stats.TotalDeliveries > 0 {
		stats.SuccessRate = float64(stats.SuccessDeliveries) / float64(stats.TotalDeliveries) * 100
	}

	return &stats, nil
}

func (r *WebhookDeliveryRepository) DeleteOlderThan(before time.Time) (int64, error) {
	result := r.db.Where("created_at < ?", before).Delete(&models.WebhookDelivery{})
	return result.RowsAffected, result.Error
}
