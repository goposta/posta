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
	"net/mail"
	"strings"

	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// normalizeEmail extracts the bare email address from a string that may be in
// RFC 5322 format like "Display Name <user@example.com>" and lowercases it.
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

// Upsert adds a suppression idempotently. If a row already exists for the same
// (user_id, workspace_id, email), the existing row is kept and no error is
// returned. Intended for automated paths (bounces, one-click unsubscribe).
func (r *SuppressionRepository) Upsert(suppression *models.Suppression) error {
	suppression.Email = normalizeEmail(suppression.Email)
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(suppression).Error
}

// Delete removes a suppression for an email within scope.
func (r *SuppressionRepository) Delete(scope ResourceScope, email string, listID *uint) error {
	q := ApplyScope(r.db, scope).Where("email = ?", normalizeEmail(email))
	if listID != nil {
		q = q.Where("list_id = ?", *listID)
	} else {
		q = q.Where("list_id IS NULL")
	}
	return q.Delete(&models.Suppression{}).Error
}

// applyListPredicate scopes a suppression query to what blocks a send for the
// given list. listID nil ⇒ only global rows apply (list_id IS NULL). A set listID
// ⇒ a global block OR that list's opt-out applies (a global block always wins).
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

// IsSuppressed reports whether email is suppressed at the global scope (no list).
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

// FilterSuppressedForList drops recipients blocked for the given list (global
// block, or that list's opt-out when listID is set).
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
