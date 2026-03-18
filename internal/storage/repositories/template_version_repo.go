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

type TemplateVersionRepository struct {
	db *gorm.DB
}

func NewTemplateVersionRepository(db *gorm.DB) *TemplateVersionRepository {
	return &TemplateVersionRepository{db: db}
}

func (r *TemplateVersionRepository) Create(v *models.TemplateVersion) error {
	return r.db.Create(v).Error
}

func (r *TemplateVersionRepository) FindByID(id uint) (*models.TemplateVersion, error) {
	var v models.TemplateVersion
	if err := r.db.Preload("StyleSheet").First(&v, id).Error; err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *TemplateVersionRepository) FindByTemplateID(templateID uint) ([]models.TemplateVersion, error) {
	var versions []models.TemplateVersion
	if err := r.db.Preload("StyleSheet").Preload("Localizations").
		Where("template_id = ?", templateID).
		Order("version DESC").
		Find(&versions).Error; err != nil {
		return nil, err
	}
	return versions, nil
}

func (r *TemplateVersionRepository) NextVersion(templateID uint) (int, error) {
	var max int
	err := r.db.Model(&models.TemplateVersion{}).
		Where("template_id = ?", templateID).
		Select("COALESCE(MAX(version), 0)").
		Scan(&max).Error
	return max + 1, err
}

func (r *TemplateVersionRepository) Update(v *models.TemplateVersion) error {
	return r.db.Save(v).Error
}

func (r *TemplateVersionRepository) Delete(id uint) error {
	return r.db.Delete(&models.TemplateVersion{}, id).Error
}
