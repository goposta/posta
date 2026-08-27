// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package repositories

import (
	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
)

type LanguageRepository struct {
	db *gorm.DB
}

func NewLanguageRepository(db *gorm.DB) *LanguageRepository {
	return &LanguageRepository{db: db}
}

func (r *LanguageRepository) Create(l *models.Language) error {
	return r.db.Create(l).Error
}

func (r *LanguageRepository) Update(l *models.Language) error {
	return r.db.Save(l).Error
}

func (r *LanguageRepository) Delete(id uint) error {
	return r.db.Delete(&models.Language{}, id).Error
}

func (r *LanguageRepository) FindByID(id uint) (*models.Language, error) {
	var l models.Language
	if err := r.db.First(&l, id).Error; err != nil {
		return nil, err
	}
	return &l, nil
}

func (r *LanguageRepository) FindByWorkspaceID(workspaceID uint, limit, offset int) ([]models.Language, int64, error) {
	var languages []models.Language
	var total int64

	r.db.Model(&models.Language{}).Where("workspace_id = ?", workspaceID).Count(&total)

	if err := r.db.Where("workspace_id = ?", workspaceID).
		Order("code ASC").
		Limit(limit).Offset(offset).
		Find(&languages).Error; err != nil {
		return nil, 0, err
	}
	return languages, total, nil
}

func (r *LanguageRepository) FindByUserID(userID uint, limit, offset int) ([]models.Language, int64, error) {
	var languages []models.Language
	var total int64

	r.db.Model(&models.Language{}).Where("user_id = ? AND workspace_id IS NULL", userID).Count(&total)

	if err := r.db.Where("user_id = ? AND workspace_id IS NULL", userID).
		Order("code ASC").
		Limit(limit).Offset(offset).
		Find(&languages).Error; err != nil {
		return nil, 0, err
	}
	return languages, total, nil
}

// ClearDefault unsets is_default for all languages in the given scope.
func (r *LanguageRepository) ClearDefault(scope ResourceScope) error {
	return ApplyScope(r.db.Model(&models.Language{}), scope).
		Where("is_default = ?", true).
		Update("is_default", false).Error
}

// FindDefault returns the default language for the given scope, or nil if none.
func (r *LanguageRepository) FindDefault(scope ResourceScope) (*models.Language, error) {
	var l models.Language
	if err := ApplyScope(r.db, scope).Where("is_default = ?", true).First(&l).Error; err != nil {
		return nil, err
	}
	return &l, nil
}

func (r *LanguageRepository) FindByScope(scope ResourceScope, limit, offset int) ([]models.Language, int64, error) {
	var items []models.Language
	var total int64

	ApplyScope(r.db.Model(&models.Language{}), scope).Count(&total)

	if err := ApplyScope(r.db, scope).
		Order("code ASC").
		Limit(limit).Offset(offset).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
