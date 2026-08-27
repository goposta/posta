// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package repositories

import (
	"github.com/goposta/posta/internal/models"
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
