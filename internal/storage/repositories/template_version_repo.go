// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package repositories

import (
	"github.com/goposta/posta/internal/models"
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
