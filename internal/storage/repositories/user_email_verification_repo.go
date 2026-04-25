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

	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
)

type UserEmailVerificationRepository struct {
	db *gorm.DB
}

func NewUserEmailVerificationRepository(db *gorm.DB) *UserEmailVerificationRepository {
	return &UserEmailVerificationRepository{db: db}
}

func (r *UserEmailVerificationRepository) Create(v *models.UserEmailVerification) error {
	return r.db.Create(v).Error
}

func (r *UserEmailVerificationRepository) FindByTokenHash(hash string) (*models.UserEmailVerification, error) {
	var v models.UserEmailVerification
	if err := r.db.Where("token_hash = ?", hash).First(&v).Error; err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *UserEmailVerificationRepository) MarkUsed(id uint) error {
	now := time.Now()
	return r.db.Model(&models.UserEmailVerification{}).
		Where("id = ?", id).
		Update("used_at", now).Error
}

// InvalidatePending marks all pending (unused) tokens for a user as used.
// Called when a new token is issued or when the email becomes verified.
func (r *UserEmailVerificationRepository) InvalidatePending(userID uint) error {
	now := time.Now()
	return r.db.Model(&models.UserEmailVerification{}).
		Where("user_id = ? AND used_at IS NULL", userID).
		Update("used_at", now).Error
}

// CountRecentByUser counts tokens created for a user since a given time (for rate limiting).
func (r *UserEmailVerificationRepository) CountRecentByUser(userID uint, since time.Time) (int64, error) {
	var count int64
	err := r.db.Model(&models.UserEmailVerification{}).
		Where("user_id = ? AND created_at > ?", userID, since).
		Count(&count).Error
	return count, err
}
