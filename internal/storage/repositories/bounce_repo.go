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

	r.db.Model(&models.Bounce{}).Where("user_id = ? AND workspace_id IS NULL", userID).Count(&total)

	if err := r.db.Where("user_id = ? AND workspace_id IS NULL", userID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&bounces).Error; err != nil {
		return nil, 0, err
	}
	return bounces, total, nil
}

func (r *BounceRepository) FindByWorkspaceID(workspaceID uint, limit, offset int) ([]models.Bounce, int64, error) {
	var bounces []models.Bounce
	var total int64

	r.db.Model(&models.Bounce{}).Where("workspace_id = ?", workspaceID).Count(&total)

	if err := r.db.Where("workspace_id = ?", workspaceID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&bounces).Error; err != nil {
		return nil, 0, err
	}
	return bounces, total, nil
}

func (r *BounceRepository) FindByScope(scope ResourceScope, limit, offset int) ([]models.Bounce, int64, error) {
	var items []models.Bounce
	var total int64

	ApplyScope(r.db.Model(&models.Bounce{}), scope).Count(&total)

	if err := ApplyScope(r.db, scope).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
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

func (r *BounceRepository) CountByUserAndDateRange(userID uint, from, to time.Time) (int64, error) {
	var count int64
	err := r.db.Model(&models.Bounce{}).
		Where("user_id = ? AND created_at >= ? AND created_at <= ?", userID, from, to).
		Count(&count).Error
	return count, err
}

func (r *BounceRepository) CountByWorkspaceAndDateRange(workspaceID uint, from, to time.Time) (int64, error) {
	var count int64
	err := r.db.Model(&models.Bounce{}).
		Where("workspace_id = ? AND created_at >= ? AND created_at <= ?", workspaceID, from, to).
		Count(&count).Error
	return count, err
}
