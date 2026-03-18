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

type SettingRepository struct {
	db *gorm.DB
}

func NewSettingRepository(db *gorm.DB) *SettingRepository {
	return &SettingRepository{db: db}
}

func (r *SettingRepository) FindAll() ([]models.Setting, error) {
	var settings []models.Setting
	if err := r.db.Order("key ASC").Find(&settings).Error; err != nil {
		return nil, err
	}
	return settings, nil
}

func (r *SettingRepository) FindByKey(key string) (*models.Setting, error) {
	var setting models.Setting
	if err := r.db.Where("key = ?", key).First(&setting).Error; err != nil {
		return nil, err
	}
	return &setting, nil
}

func (r *SettingRepository) Upsert(setting *models.Setting) error {
	var existing models.Setting
	result := r.db.Where("key = ?", setting.Key).First(&existing)
	if result.Error == nil {
		existing.Value = setting.Value
		existing.Type = setting.Type
		return r.db.Save(&existing).Error
	}
	return r.db.Create(setting).Error
}

func (r *SettingRepository) BulkUpsert(settings []models.Setting) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		repo := &SettingRepository{db: tx}
		for i := range settings {
			if err := repo.Upsert(&settings[i]); err != nil {
				return err
			}
		}
		return nil
	})
}
