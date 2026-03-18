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
	"time"

	"github.com/jkaninda/posta/internal/models"
	"gorm.io/gorm"
)

type APIKeyRepository struct {
	db *gorm.DB
}

func NewAPIKeyRepository(db *gorm.DB) *APIKeyRepository {
	return &APIKeyRepository{db: db}
}

func (r *APIKeyRepository) Create(key *models.APIKey) error {
	return r.db.Create(key).Error
}

func (r *APIKeyRepository) FindByPrefix(prefix string) ([]models.APIKey, error) {
	var keys []models.APIKey
	if err := r.db.Where("key_prefix = ?", prefix).Find(&keys).Error; err != nil {
		return nil, err
	}
	return keys, nil
}

func (r *APIKeyRepository) FindByUserID(userID uint, limit, offset int) ([]models.APIKey, int64, error) {
	var keys []models.APIKey
	var total int64

	r.db.Model(&models.APIKey{}).Where("user_id = ?", userID).Count(&total)

	if err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&keys).Error; err != nil {
		return nil, 0, err
	}
	return keys, total, nil
}

func (r *APIKeyRepository) FindByID(id uint) (*models.APIKey, error) {
	var key models.APIKey
	if err := r.db.First(&key, id).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

func (r *APIKeyRepository) UpdateLastUsed(id uint) error {
	now := time.Now()
	return r.db.Model(&models.APIKey{}).Where("id = ?", id).Update("last_used_at", now).Error
}

func (r *APIKeyRepository) Revoke(id uint) error {
	return r.db.Model(&models.APIKey{}).Where("id = ?", id).Update("revoked", true).Error
}

func (r *APIKeyRepository) Delete(id uint) error {
	return r.db.Delete(&models.APIKey{}, id).Error
}

func (r *APIKeyRepository) FindAll(limit, offset int) ([]models.APIKey, int64, error) {
	var keys []models.APIKey
	var total int64

	r.db.Model(&models.APIKey{}).Count(&total)

	if err := r.db.Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&keys).Error; err != nil {
		return nil, 0, err
	}
	return keys, total, nil
}
