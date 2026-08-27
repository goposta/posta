// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package repositories

import (
	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
)

type StyleSheetRepository struct {
	db *gorm.DB
}

func NewStyleSheetRepository(db *gorm.DB) *StyleSheetRepository {
	return &StyleSheetRepository{db: db}
}

func (r *StyleSheetRepository) Create(ss *models.StyleSheet) error {
	return r.db.Create(ss).Error
}

func (r *StyleSheetRepository) Update(ss *models.StyleSheet) error {
	return r.db.Save(ss).Error
}

func (r *StyleSheetRepository) Delete(id uint) error {
	return r.db.Delete(&models.StyleSheet{}, id).Error
}

func (r *StyleSheetRepository) FindByID(id uint) (*models.StyleSheet, error) {
	var ss models.StyleSheet
	if err := r.db.First(&ss, id).Error; err != nil {
		return nil, err
	}
	return &ss, nil
}

func (r *StyleSheetRepository) FindByWorkspaceID(workspaceID uint, limit, offset int) ([]models.StyleSheet, int64, error) {
	var sheets []models.StyleSheet
	var total int64

	r.db.Model(&models.StyleSheet{}).Where("workspace_id = ?", workspaceID).Count(&total)

	if err := r.db.Where("workspace_id = ?", workspaceID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&sheets).Error; err != nil {
		return nil, 0, err
	}
	return sheets, total, nil
}

func (r *StyleSheetRepository) FindByUserID(userID uint, limit, offset int) ([]models.StyleSheet, int64, error) {
	var sheets []models.StyleSheet
	var total int64

	r.db.Model(&models.StyleSheet{}).Where("user_id = ? AND workspace_id IS NULL", userID).Count(&total)

	if err := r.db.Where("user_id = ? AND workspace_id IS NULL", userID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&sheets).Error; err != nil {
		return nil, 0, err
	}
	return sheets, total, nil
}

func (r *StyleSheetRepository) FindByIDInScope(scope ResourceScope, id uint) (*models.StyleSheet, error) {
	var ss models.StyleSheet
	if err := ApplyScope(r.db, scope).Where("id = ?", id).First(&ss).Error; err != nil {
		return nil, err
	}
	return &ss, nil
}

func (r *StyleSheetRepository) FindByNameInScope(scope ResourceScope, name string) (*models.StyleSheet, error) {
	var ss models.StyleSheet
	if err := ApplyScope(r.db, scope).Where("name = ?", name).First(&ss).Error; err != nil {
		return nil, err
	}
	return &ss, nil
}

func (r *StyleSheetRepository) FindByScope(scope ResourceScope, limit, offset int) ([]models.StyleSheet, int64, error) {
	var items []models.StyleSheet
	var total int64

	ApplyScope(r.db.Model(&models.StyleSheet{}), scope).Count(&total)

	if err := ApplyScope(r.db, scope).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
