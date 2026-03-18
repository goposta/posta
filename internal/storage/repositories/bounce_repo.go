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

type BounceRepository struct {
	db *gorm.DB
}

func NewBounceRepository(db *gorm.DB) *BounceRepository {
	return &BounceRepository{db: db}
}

func (r *BounceRepository) Create(bounce *models.Bounce) error {
	return r.db.Create(bounce).Error
}

func (r *BounceRepository) FindByUserID(userID uint, limit, offset int) ([]models.Bounce, int64, error) {
	var bounces []models.Bounce
	var total int64

	r.db.Model(&models.Bounce{}).Where("user_id = ?", userID).Count(&total)

	if err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&bounces).Error; err != nil {
		return nil, 0, err
	}
	return bounces, total, nil
}

func (r *BounceRepository) FindByEmailID(emailID uint) ([]models.Bounce, error) {
	var bounces []models.Bounce
	if err := r.db.Where("email_id = ?", emailID).Find(&bounces).Error; err != nil {
		return nil, err
	}
	return bounces, nil
}

func (r *BounceRepository) CountHardBouncesByRecipient(userID uint, recipient string) (int64, error) {
	var count int64
	err := r.db.Model(&models.Bounce{}).
		Where("user_id = ? AND recipient = ? AND type = ?", userID, recipient, models.BounceTypeHard).
		Count(&count).Error
	return count, err
}
