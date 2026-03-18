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
	"github.com/jkaninda/posta/internal/models"
	"gorm.io/gorm"
)

type WebhookRepository struct {
	db *gorm.DB
}

func NewWebhookRepository(db *gorm.DB) *WebhookRepository {
	return &WebhookRepository{db: db}
}

func (r *WebhookRepository) Create(webhook *models.Webhook) error {
	return r.db.Create(webhook).Error
}

func (r *WebhookRepository) Delete(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("webhook_id = ?", id).Delete(&models.WebhookDelivery{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.Webhook{}, id).Error
	})
}

func (r *WebhookRepository) FindByID(id uint) (*models.Webhook, error) {
	var webhook models.Webhook
	if err := r.db.First(&webhook, id).Error; err != nil {
		return nil, err
	}
	return &webhook, nil
}

func (r *WebhookRepository) FindByUserID(userID uint, limit, offset int) ([]models.Webhook, int64, error) {
	var webhooks []models.Webhook
	var total int64

	r.db.Model(&models.Webhook{}).Where("user_id = ?", userID).Count(&total)

	if err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&webhooks).Error; err != nil {
		return nil, 0, err
	}
	return webhooks, total, nil
}

func (r *WebhookRepository) FindByUserIDAndEvent(userID uint, event string) ([]models.Webhook, error) {
	var webhooks []models.Webhook
	if err := r.db.Where("user_id = ? AND ? = ANY(events)", userID, event).Find(&webhooks).Error; err != nil {
		return nil, err
	}
	return webhooks, nil
}
