// SPDX-FileCopyrightText: 2026 Jonas Kaninda
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

// CountByUserAndDateRange counts bounces for a user within a date range.
func (r *BounceRepository) CountByUserAndDateRange(userID uint, from, to time.Time) (int64, error) {
	var count int64
	err := r.db.Model(&models.Bounce{}).
		Where("user_id = ? AND created_at >= ? AND created_at <= ?", userID, from, to).
		Count(&count).Error
	return count, err
}

// CountByWorkspaceAndDateRange counts bounces for a workspace within a date range.
func (r *BounceRepository) CountByWorkspaceAndDateRange(workspaceID uint, from, to time.Time) (int64, error) {
	var count int64
	err := r.db.Model(&models.Bounce{}).
		Where("workspace_id = ? AND created_at >= ? AND created_at <= ?", workspaceID, from, to).
		Count(&count).Error
	return count, err
}
