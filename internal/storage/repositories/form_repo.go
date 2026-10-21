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
	"gorm.io/gorm/clause"
)

type FormRepository struct {
	db *gorm.DB
}

func NewFormRepository(db *gorm.DB) *FormRepository {
	return &FormRepository{db: db}
}

func (r *FormRepository) Create(f *models.Form) error {
	return r.db.Create(f).Error
}

func (r *FormRepository) Update(f *models.Form) error {
	return r.db.Omit(clause.Associations).Save(f).Error
}

func (r *FormRepository) FindByID(id uint) (*models.Form, error) {
	var f models.Form
	if err := r.db.Preload("LastEditedBy").First(&f, id).Error; err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *FormRepository) FindByIDForScope(scope ResourceScope, id uint) (*models.Form, error) {
	var f models.Form
	if err := ApplyWorkspaceScope(r.db, scope).Preload("LastEditedBy").First(&f, id).Error; err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *FormRepository) FindByPublicKey(key string) (*models.Form, error) {
	var f models.Form
	if err := r.db.Where("public_key = ?", key).First(&f).Error; err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *FormRepository) FindByScope(scope ResourceScope, limit, offset int) ([]models.Form, int64, error) {
	var items []models.Form
	var total int64

	ApplyWorkspaceScope(r.db.Model(&models.Form{}), scope).Count(&total)

	if err := ApplyWorkspaceScope(r.db, scope).
		Preload("LastEditedBy").
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *FormRepository) CountByScope(scope ResourceScope) (int64, error) {
	var total int64
	err := ApplyWorkspaceScope(r.db.Model(&models.Form{}), scope).Count(&total).Error
	return total, err
}

func (r *FormRepository) Delete(id uint) error {
	return r.db.Delete(&models.Form{}, id).Error
}

func (r *FormRepository) SlugExists(workspaceID uint, slug string, excludeID uint) bool {
	var count int64
	q := r.db.Model(&models.Form{}).Where("workspace_id = ? AND slug = ?", workspaceID, slug)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	q.Count(&count)
	return count > 0
}

func (r *FormRepository) RecordSubmission(id uint, spam bool, at time.Time) error {
	updates := map[string]any{
		"message_count":   gorm.Expr("message_count + 1"),
		"last_message_at": at,
	}
	if spam {
		updates["spam_count"] = gorm.Expr("spam_count + 1")
	}
	return r.db.Model(&models.Form{}).Where("id = ?", id).Updates(updates).Error
}

func (r *FormRepository) FindActiveWithDigest(mode models.NotifyMode) ([]models.Form, error) {
	var items []models.Form
	err := r.db.Where("status = ? AND notify_enabled = true AND notify_mode = ?", models.FormStatusActive, mode).
		Find(&items).Error
	return items, err
}

// FindWithRetention returns the forms that set their own message retention.
func (r *FormRepository) FindWithRetention() ([]models.Form, error) {
	var items []models.Form
	err := r.db.Where("retention_days > 0").Find(&items).Error
	return items, err
}
