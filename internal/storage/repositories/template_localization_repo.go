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

type TemplateLocalizationRepository struct {
	db *gorm.DB
}

func NewTemplateLocalizationRepository(db *gorm.DB) *TemplateLocalizationRepository {
	return &TemplateLocalizationRepository{db: db}
}

func (r *TemplateLocalizationRepository) Create(l *models.TemplateLocalization) error {
	return r.db.Create(l).Error
}

func (r *TemplateLocalizationRepository) Update(l *models.TemplateLocalization) error {
	return r.db.Save(l).Error
}

func (r *TemplateLocalizationRepository) FindByID(id uint) (*models.TemplateLocalization, error) {
	var l models.TemplateLocalization
	if err := r.db.First(&l, id).Error; err != nil {
		return nil, err
	}
	return &l, nil
}

func (r *TemplateLocalizationRepository) FindByVersionID(versionID uint) ([]models.TemplateLocalization, error) {
	var localizations []models.TemplateLocalization
	if err := r.db.Where("version_id = ?", versionID).
		Order("language ASC").
		Find(&localizations).Error; err != nil {
		return nil, err
	}
	return localizations, nil
}

func (r *TemplateLocalizationRepository) FindByVersionAndLanguage(versionID uint, language string) (*models.TemplateLocalization, error) {
	var l models.TemplateLocalization
	if err := r.db.Where("version_id = ? AND language = ?", versionID, language).First(&l).Error; err != nil {
		return nil, err
	}
	return &l, nil
}

func (r *TemplateLocalizationRepository) Delete(id uint) error {
	return r.db.Delete(&models.TemplateLocalization{}, id).Error
}
