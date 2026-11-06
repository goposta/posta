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
	"net/mail"
	"strings"

	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// normalizeEmail lowercases the bare address of an RFC 5322 "Display Name <user@example.com>" string.
func normalizeEmail(raw string) string {
	addr, err := mail.ParseAddress(raw)
	if err != nil {
		return strings.ToLower(strings.TrimSpace(raw))
	}
	return strings.ToLower(addr.Address)
}

type SuppressionRepository struct {
	db *gorm.DB
}

func NewSuppressionRepository(db *gorm.DB) *SuppressionRepository {
	return &SuppressionRepository{db: db}
}

func (r *SuppressionRepository) Create(suppression *models.Suppression) error {
	suppression.Email = normalizeEmail(suppression.Email)
	return r.db.Create(suppression).Error
}

// Upsert keeps an existing (user_id, workspace_id, email) row without error, for automated paths like bounces.
func (r *SuppressionRepository) Upsert(suppression *models.Suppression) error {
	suppression.Email = normalizeEmail(suppression.Email)
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(suppression).Error
}

func (r *SuppressionRepository) Delete(scope ResourceScope, email string, listID *uint) error {
	q := ApplyScope(r.db, scope).Where("email = ?", normalizeEmail(email))
	if listID != nil {
		q = q.Where("list_id = ?", *listID)
	} else {
		q = q.Where("list_id IS NULL")
	}
	return q.Delete(&models.Suppression{}).Error
}

// applyListPredicate applies global rows, plus that list's opt-out when listID is set; a global block always wins.
func applyListPredicate(db *gorm.DB, listID *uint) *gorm.DB {
	if listID != nil {
		// Explicit parens so the OR can't bind loosely against the scope/email ANDs.
		return db.Where("(list_id IS NULL OR list_id = ?)", *listID)
	}
	return db.Where("list_id IS NULL")
}

func (r *SuppressionRepository) FindByUserID(userID uint, limit, offset int) ([]models.Suppression, int64, error) {
	var suppressions []models.Suppression
	var total int64

	r.db.Model(&models.Suppression{}).Where("user_id = ? AND workspace_id IS NULL", userID).Count(&total)

	if err := r.db.Where("user_id = ? AND workspace_id IS NULL", userID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&suppressions).Error; err != nil {
		return nil, 0, err
	}
	return suppressions, total, nil
}

func (r *SuppressionRepository) FindByWorkspaceID(workspaceID uint, limit, offset int) ([]models.Suppression, int64, error) {
	var suppressions []models.Suppression
	var total int64

	r.db.Model(&models.Suppression{}).Where("workspace_id = ?", workspaceID).Count(&total)

	if err := r.db.Where("workspace_id = ?", workspaceID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&suppressions).Error; err != nil {
		return nil, 0, err
	}
	return suppressions, total, nil
}

func (r *SuppressionRepository) FindByScope(scope ResourceScope, limit, offset int) ([]models.Suppression, int64, error) {
	var items []models.Suppression
	var total int64

	ApplyScope(r.db.Model(&models.Suppression{}), scope).Count(&total)

	if err := ApplyScope(r.db, scope).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *SuppressionRepository) FindByScopeFiltered(scope ResourceScope, listID *uint, limit, offset int) ([]models.Suppression, int64, error) {
	var items []models.Suppression
	var total int64

	base := ApplyScope(r.db.Model(&models.Suppression{}), scope)
	if listID != nil {
		base = base.Where("list_id = ?", *listID)
	}
	base.Count(&total)

	q := ApplyScope(r.db, scope)
	if listID != nil {
		q = q.Where("list_id = ?", *listID)
	}
	if err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *SuppressionRepository) IsSuppressed(scope ResourceScope, email string) (bool, error) {
	return r.IsSuppressedForList(scope, email, nil)
}

func (r *SuppressionRepository) IsSuppressedForList(scope ResourceScope, email string, listID *uint) (bool, error) {
	var count int64
	err := applyListPredicate(ApplyScope(r.db.Model(&models.Suppression{}), scope), listID).
		Where("email = ?", normalizeEmail(email)).
		Count(&count).Error
	return count > 0, err
}

func (r *SuppressionRepository) FilterSuppressed(scope ResourceScope, emails []string) ([]string, error) {
	return r.FilterSuppressedForList(scope, emails, nil)
}

// FilterSuppressedForList drops recipients blocked globally or opted out of listID.
func (r *SuppressionRepository) FilterSuppressedForList(scope ResourceScope, emails []string, listID *uint) ([]string, error) {
	if len(emails) == 0 {
		return emails, nil
	}

	lowered := make([]string, len(emails))
	for i, e := range emails {
		lowered[i] = normalizeEmail(e)
	}

	var suppressed []string
	if err := applyListPredicate(ApplyScope(r.db.Model(&models.Suppression{}), scope), listID).
		Where("email IN ?", lowered).
		Pluck("email", &suppressed).Error; err != nil {
		return nil, err
	}

	suppressedSet := make(map[string]bool, len(suppressed))
	for _, s := range suppressed {
		suppressedSet[s] = true
	}

	var filtered []string
	for _, e := range emails {
		if !suppressedSet[normalizeEmail(e)] {
			filtered = append(filtered, e)
		}
	}
	return filtered, nil
}
