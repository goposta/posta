/*
 *  MIT License
 *
 * Copyright (c) 2026 Jonas Kaninda
 *
 *  Permission is hereby granted, free of charge, to any person obtaining a copy
 *  of this software and associated documentation files (the "Software"), to deal
 *  in the Software without restriction, including without limitation the rights
 *  to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 *  copies of the Software, and to permit persons to whom the Software is
 *  furnished to do so, subject to the following conditions:
 *
 *  The above copyright notice and this permission notice shall be included in all
 *  copies or substantial portions of the Software.
 *
 *  THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 *  IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 *  FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 *  AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 *  LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 *  OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 *  SOFTWARE.
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
