// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package repositories

import (
	"github.com/goposta/posta/internal/models"
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

	r.db.Model(&models.Webhook{}).Where("user_id = ? AND workspace_id IS NULL", userID).Count(&total)

	if err := r.db.Where("user_id = ? AND workspace_id IS NULL", userID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&webhooks).Error; err != nil {
		return nil, 0, err
	}
	return webhooks, total, nil
}

func (r *WebhookRepository) FindByWorkspaceID(workspaceID uint, limit, offset int) ([]models.Webhook, int64, error) {
	var webhooks []models.Webhook
	var total int64

	r.db.Model(&models.Webhook{}).Where("workspace_id = ?", workspaceID).Count(&total)

	if err := r.db.Where("workspace_id = ?", workspaceID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&webhooks).Error; err != nil {
		return nil, 0, err
	}
	return webhooks, total, nil
}

func (r *WebhookRepository) FindByScope(scope ResourceScope, limit, offset int) ([]models.Webhook, int64, error) {
	var items []models.Webhook
	var total int64

	ApplyScope(r.db.Model(&models.Webhook{}), scope).Count(&total)

	if err := ApplyScope(r.db, scope).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *WebhookRepository) FindByScopeAndEvent(scope ResourceScope, event string) ([]models.Webhook, error) {
	var webhooks []models.Webhook
	if err := ApplyScope(r.db.Model(&models.Webhook{}), scope).
		Where("? = ANY(events)", event).
		Find(&webhooks).Error; err != nil {
		return nil, err
	}
	return webhooks, nil
}
