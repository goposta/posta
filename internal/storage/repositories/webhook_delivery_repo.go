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

	"github.com/jkaninda/posta/internal/models"
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

// WebhookDeliveryStats holds aggregate delivery counts.
type WebhookDeliveryStats struct {
	TotalDeliveries   int64   `json:"total_deliveries"`
	SuccessDeliveries int64   `json:"success_deliveries"`
	FailedDeliveries  int64   `json:"failed_deliveries"`
	SuccessRate       float64 `json:"success_rate"`
}

// StatsByUserID returns aggregate webhook delivery stats for a user.
func (r *WebhookDeliveryRepository) StatsByUserID(userID uint) (*WebhookDeliveryStats, error) {
	var stats WebhookDeliveryStats

	r.db.Model(&models.WebhookDelivery{}).Where("user_id = ?", userID).Count(&stats.TotalDeliveries)
	r.db.Model(&models.WebhookDelivery{}).Where("user_id = ? AND status = ?", userID, models.WebhookDeliverySuccess).Count(&stats.SuccessDeliveries)
	r.db.Model(&models.WebhookDelivery{}).Where("user_id = ? AND status = ?", userID, models.WebhookDeliveryFailed).Count(&stats.FailedDeliveries)

	if stats.TotalDeliveries > 0 {
		stats.SuccessRate = float64(stats.SuccessDeliveries) / float64(stats.TotalDeliveries) * 100
	}

	return &stats, nil
}

// StatsAll returns aggregate webhook delivery stats across all users.
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

// RecentByUserID returns the most recent deliveries for a user.
func (r *WebhookDeliveryRepository) RecentByUserID(userID uint, limit int) ([]models.WebhookDelivery, error) {
	var deliveries []models.WebhookDelivery
	err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Find(&deliveries).Error
	return deliveries, err
}

// FindByUserID returns paginated webhook deliveries for a user.
func (r *WebhookDeliveryRepository) FindByUserID(userID uint, limit, offset int) ([]models.WebhookDelivery, int64, error) {
	var deliveries []models.WebhookDelivery
	var total int64
	r.db.Model(&models.WebhookDelivery{}).Where("user_id = ?", userID).Count(&total)
	err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&deliveries).Error
	return deliveries, total, err
}

// DeleteOlderThan deletes webhook delivery records older than the given time.
// Returns the number of rows deleted.
func (r *WebhookDeliveryRepository) DeleteOlderThan(before time.Time) (int64, error) {
	result := r.db.Where("created_at < ?", before).Delete(&models.WebhookDelivery{})
	return result.RowsAffected, result.Error
}
