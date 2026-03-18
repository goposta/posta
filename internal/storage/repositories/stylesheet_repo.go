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

func (r *StyleSheetRepository) FindByUserID(userID uint, limit, offset int) ([]models.StyleSheet, int64, error) {
	var sheets []models.StyleSheet
	var total int64

	r.db.Model(&models.StyleSheet{}).Where("user_id = ?", userID).Count(&total)

	if err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&sheets).Error; err != nil {
		return nil, 0, err
	}
	return sheets, total, nil
}
