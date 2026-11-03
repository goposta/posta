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
	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

// FindByNameInScope finds a stylesheet by name or, for exports and HTML
// files made before 0.18, when a stylesheet's name was its label, by
// display name.
func (r *StyleSheetRepository) FindByNameInScope(scope ResourceScope, name string) (*models.StyleSheet, error) {
	var ss models.StyleSheet
	if err := ApplyScope(r.db, scope).Where("name = ? OR display_name = ?", name, name).
		Order(clause.OrderBy{Expression: clause.Expr{SQL: "name = ? DESC, id", Vars: []any{name}}}).
		First(&ss).Error; err != nil {
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
