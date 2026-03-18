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

func (r *LanguageRepository) FindByUserID(userID uint, limit, offset int) ([]models.Language, int64, error) {
	var languages []models.Language
	var total int64

	r.db.Model(&models.Language{}).Where("user_id = ?", userID).Count(&total)

	if err := r.db.Where("user_id = ?", userID).
		Order("code ASC").
		Limit(limit).Offset(offset).
		Find(&languages).Error; err != nil {
		return nil, 0, err
	}
	return languages, total, nil
}
