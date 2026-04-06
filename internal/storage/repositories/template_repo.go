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
	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
)

type TemplateRepository struct {
	db *gorm.DB
}

func NewTemplateRepository(db *gorm.DB) *TemplateRepository {
	return &TemplateRepository{db: db}
}

func (r *TemplateRepository) Create(tmpl *models.Template) error {
	return r.db.Create(tmpl).Error
}

func (r *TemplateRepository) Update(tmpl *models.Template) error {
	return r.db.Save(tmpl).Error
}

func (r *TemplateRepository) Delete(id uint) error {
	return r.db.Delete(&models.Template{}, id).Error
}

func (r *TemplateRepository) FindByID(id uint) (*models.Template, error) {
	var tmpl models.Template
	if err := r.db.First(&tmpl, id).Error; err != nil {
		return nil, err
	}
	return &tmpl, nil
}

func (r *TemplateRepository) FindByName(userID uint, name string) (*models.Template, error) {
	var tmpl models.Template
	if err := r.db.Where("user_id = ? AND name = ? AND workspace_id IS NULL", userID, name).First(&tmpl).Error; err != nil {
		return nil, err
	}
	return &tmpl, nil
}

func (r *TemplateRepository) FindByUserID(userID uint, limit, offset int) ([]models.Template, int64, error) {
	var templates []models.Template
	var total int64

	r.db.Model(&models.Template{}).Where("user_id = ? AND workspace_id IS NULL", userID).Count(&total)

	if err := r.db.Where("user_id = ? AND workspace_id IS NULL", userID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&templates).Error; err != nil {
		return nil, 0, err
	}
	return templates, total, nil
}

func (r *TemplateRepository) FindByWorkspaceID(workspaceID uint, limit, offset int) ([]models.Template, int64, error) {
	var templates []models.Template
	var total int64

	r.db.Model(&models.Template{}).Where("workspace_id = ?", workspaceID).Count(&total)

	if err := r.db.Where("workspace_id = ?", workspaceID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&templates).Error; err != nil {
		return nil, 0, err
	}
	return templates, total, nil
}

func (r *TemplateRepository) FindByScope(scope ResourceScope, search string, limit, offset int) ([]models.Template, int64, error) {
	var items []models.Template
	var total int64

	q := ApplyScope(r.db.Model(&models.Template{}), scope)
	if search != "" {
		q = q.Where("name ILIKE ? OR description ILIKE ?", "%"+search+"%", "%"+search+"%")
	}
	q.Count(&total)

	qItems := ApplyScope(r.db, scope)
	if search != "" {
		qItems = qItems.Where("name ILIKE ? OR description ILIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if err := qItems.
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *TemplateRepository) FindByWorkspaceName(workspaceID uint, name string) (*models.Template, error) {
	var tmpl models.Template
	if err := r.db.Where("workspace_id = ? AND name = ?", workspaceID, name).First(&tmpl).Error; err != nil {
		return nil, err
	}
	return &tmpl, nil
}
