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
	if err := r.db.Where("user_id = ? AND name = ?", userID, name).First(&tmpl).Error; err != nil {
		return nil, err
	}
	return &tmpl, nil
}

func (r *TemplateRepository) FindByUserID(userID uint, limit, offset int) ([]models.Template, int64, error) {
	var templates []models.Template
	var total int64

	r.db.Model(&models.Template{}).Where("user_id = ?", userID).Count(&total)

	if err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&templates).Error; err != nil {
		return nil, 0, err
	}
	return templates, total, nil
}
