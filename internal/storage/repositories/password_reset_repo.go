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
	"time"

	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
)

type PasswordResetRepository struct {
	db *gorm.DB
}

func NewPasswordResetRepository(db *gorm.DB) *PasswordResetRepository {
	return &PasswordResetRepository{db: db}
}

func (r *PasswordResetRepository) Create(t *models.PasswordResetToken) error {
	return r.db.Create(t).Error
}

func (r *PasswordResetRepository) FindByTokenHash(hash string) (*models.PasswordResetToken, error) {
	var t models.PasswordResetToken
	if err := r.db.Where("token_hash = ?", hash).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *PasswordResetRepository) MarkUsed(id uint) error {
	now := time.Now()
	return r.db.Model(&models.PasswordResetToken{}).
		Where("id = ?", id).
		Update("used_at", now).Error
}

// InvalidatePending marks unused tokens used; called when a new token is issued and when a reset completes.
func (r *PasswordResetRepository) InvalidatePending(userID uint) error {
	now := time.Now()
	return r.db.Model(&models.PasswordResetToken{}).
		Where("user_id = ? AND used_at IS NULL", userID).
		Update("used_at", now).Error
}

// CountRecentByUser counts tokens created for a user since a given time (for rate limiting).
func (r *PasswordResetRepository) CountRecentByUser(userID uint, since time.Time) (int64, error) {
	var count int64
	err := r.db.Model(&models.PasswordResetToken{}).
		Where("user_id = ? AND created_at > ?", userID, since).
		Count(&count).Error
	return count, err
}
