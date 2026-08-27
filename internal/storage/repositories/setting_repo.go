// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package repositories

import (
	"github.com/goposta/posta/internal/models"
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
