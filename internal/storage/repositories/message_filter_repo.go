// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package repositories

import (
	"time"

	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
)

type MessageFilterRepository struct {
	db *gorm.DB
}

func NewMessageFilterRepository(db *gorm.DB) *MessageFilterRepository {
	return &MessageFilterRepository{db: db}
}

func (r *MessageFilterRepository) Create(f *models.MessageFilter) error {
	return r.db.Create(f).Error
}

func (r *MessageFilterRepository) Update(f *models.MessageFilter) error {
	return r.db.Save(f).Error
}

func (r *MessageFilterRepository) FindByIDForScope(scope ResourceScope, id uint) (*models.MessageFilter, error) {
	var f models.MessageFilter
	if err := ApplyWorkspaceScope(r.db, scope).First(&f, id).Error; err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *MessageFilterRepository) FindByScope(scope ResourceScope, limit, offset int) ([]models.MessageFilter, int64, error) {
	var items []models.MessageFilter
	var total int64

	ApplyWorkspaceScope(r.db.Model(&models.MessageFilter{}), scope).Count(&total)

	if err := ApplyWorkspaceScope(r.db, scope).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *MessageFilterRepository) FindActive(workspaceID uint, formID uint) ([]models.MessageFilter, error) {
	var items []models.MessageFilter
	err := r.db.Where("workspace_id = ? AND enabled = true AND (form_id IS NULL OR form_id = ?)", workspaceID, formID).
		Order("action DESC, id ASC").
		Find(&items).Error
	return items, err
}

func (r *MessageFilterRepository) Delete(id uint) error {
	return r.db.Delete(&models.MessageFilter{}, id).Error
}

func (r *MessageFilterRepository) RecordHits(ids []uint, at time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.Model(&models.MessageFilter{}).Where("id IN ?", ids).Updates(map[string]any{
		"hit_count":   gorm.Expr("hit_count + 1"),
		"last_hit_at": at,
	}).Error
}

func (r *MessageFilterRepository) CountByWorkspace(workspaceID uint) int64 {
	var count int64
	r.db.Model(&models.MessageFilter{}).Where("workspace_id = ?", workspaceID).Count(&count)
	return count
}
