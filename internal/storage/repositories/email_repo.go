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

type EmailRepository struct {
	db *gorm.DB
}

func NewEmailRepository(db *gorm.DB) *EmailRepository {
	return &EmailRepository{db: db}
}

func (r *EmailRepository) Create(email *models.Email) error {
	return r.db.Create(email).Error
}

func (r *EmailRepository) Update(email *models.Email) error {
	return r.db.Save(email).Error
}

func (r *EmailRepository) FindByID(id uint) (*models.Email, error) {
	var email models.Email
	if err := r.db.First(&email, id).Error; err != nil {
		return nil, err
	}
	return &email, nil
}

func (r *EmailRepository) FindByUUID(uuid string) (*models.Email, error) {
	var email models.Email
	if err := r.db.Where("uuid = ?", uuid).First(&email).Error; err != nil {
		return nil, err
	}
	return &email, nil
}

func (r *EmailRepository) FindByUUIDWithAPIKey(uuid string) (*models.Email, error) {
	var email models.Email
	if err := r.db.Preload("APIKey").Where("uuid = ?", uuid).First(&email).Error; err != nil {
		return nil, err
	}
	return &email, nil
}

func (r *EmailRepository) FindByUserID(userID uint, limit, offset int) ([]models.Email, int64, error) {
	var emails []models.Email
	var total int64

	r.db.Model(&models.Email{}).Where("user_id = ? AND workspace_id IS NULL", userID).Count(&total)

	if err := r.db.Where("user_id = ? AND workspace_id IS NULL", userID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&emails).Error; err != nil {
		return nil, 0, err
	}
	return emails, total, nil
}

func (r *EmailRepository) FindByWorkspaceID(workspaceID uint, limit, offset int) ([]models.Email, int64, error) {
	var emails []models.Email
	var total int64

	r.db.Model(&models.Email{}).Where("workspace_id = ?", workspaceID).Count(&total)

	if err := r.db.Where("workspace_id = ?", workspaceID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&emails).Error; err != nil {
		return nil, 0, err
	}
	return emails, total, nil
}

func (r *EmailRepository) FindByScope(scope ResourceScope, limit, offset int) ([]models.Email, int64, error) {
	var items []models.Email
	var total int64

	ApplyScope(r.db.Model(&models.Email{}), scope).Count(&total)

	if err := ApplyScope(r.db, scope).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// FindByScopeFiltered sorts by an optional key like "-sent_at"; empty or unknown keeps newest-first.
func (r *EmailRepository) FindByScopeFiltered(scope ResourceScope, f EmailFilter, sort string, limit, offset int) ([]models.Email, int64, error) {
	var items []models.Email
	var total int64

	apply := func(q *gorm.DB) *gorm.DB {
		q = ApplyScope(q, scope)
		if f.Sender != "" {
			q = q.Where("LOWER(sender) LIKE ?", likePattern(f.Sender))
		}
		if f.Recipient != "" {
			q = q.Where("EXISTS (SELECT 1 FROM unnest(recipients) AS r WHERE LOWER(r) LIKE ?)",
				likePattern(f.Recipient))
		}
		if f.Subject != "" {
			q = q.Where("LOWER(subject) LIKE ?", likePattern(f.Subject))
		}
		if f.Text != "" {
			pat := likePattern(f.Text)
			q = q.Where("(LOWER(subject) LIKE ? OR EXISTS (SELECT 1 FROM unnest(recipients) AS r WHERE LOWER(r) LIKE ?))",
				pat, pat)
		}
		if f.Template != "" {
			q = q.Where("LOWER(template_name) LIKE ?", likePattern(f.Template))
		}
		if len(f.Providers) > 0 {
			cond, args := likeAny("LOWER(provider)", f.Providers)
			q = q.Where(cond, args...)
		}
		if len(f.Servers) > 0 {
			cond, args := likeAny("LOWER(smtp_hostname)", f.Servers)
			q = q.Where(cond, args...)
		}
		if len(f.APIKeys) > 0 {
			cond, args := likeAny("LOWER(k.name)", f.APIKeys)
			q = q.Where("EXISTS (SELECT 1 FROM api_keys k WHERE k.id = emails.api_key_id AND "+cond+")", args...)
		}
		if len(f.Statuses) > 0 {
			q = q.Where("status IN ?", f.Statuses)
		}
		if f.HasAttachment {
			q = q.Where("attachments_json <> '' AND attachments_json <> '[]'")
		}
		if f.After != nil {
			q = q.Where("created_at >= ?", *f.After)
		}
		if f.Before != nil {
			q = q.Where("created_at < ?", *f.Before)
		}
		return q
	}

	apply(r.db.Model(&models.Email{})).Count(&total)

	if err := apply(r.db).
		Order(orderClause(sort)).
		Limit(limit).Offset(offset).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *EmailRepository) FindFailedForRetry(userID uint, maxRetries int) ([]models.Email, error) {
	var emails []models.Email
	if err := r.db.Where("user_id = ? AND status = ? AND retry_count < ?", userID, models.EmailStatusFailed, maxRetries).
		Order("created_at ASC").
		Find(&emails).Error; err != nil {
		return nil, err
	}
	return emails, nil
}

func (r *EmailRepository) FindFailedForRetryByWorkspace(workspaceID uint, maxRetries int) ([]models.Email, error) {
	var emails []models.Email
	if err := r.db.Where("workspace_id = ? AND status = ? AND retry_count < ?", workspaceID, models.EmailStatusFailed, maxRetries).
		Order("created_at ASC").
		Find(&emails).Error; err != nil {
		return nil, err
	}
	return emails, nil
}

func (r *EmailRepository) FindAll(limit, offset int) ([]models.Email, int64, error) {
	var emails []models.Email
	var total int64

	r.db.Model(&models.Email{}).Count(&total)

	if err := r.db.Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&emails).Error; err != nil {
		return nil, 0, err
	}
	return emails, total, nil
}

func (r *EmailRepository) DeleteOlderThan(before time.Time) (int64, error) {
	result := r.db.Where("created_at < ?", before).Delete(&models.Email{})
	return result.RowsAffected, result.Error
}

// AttachmentsJSONOlderThan lets the caller delete blob-storage keys before the rows are dropped.
func (r *EmailRepository) AttachmentsJSONOlderThan(before time.Time) ([]string, error) {
	var out []string
	err := r.db.Model(&models.Email{}).
		Where("created_at < ? AND attachments_json <> ''", before).
		Pluck("attachments_json", &out).Error
	return out, err
}

// ScrubBodiesOlderThan clears bodies but keeps the record.
func (r *EmailRepository) ScrubBodiesOlderThan(before time.Time) (int64, error) {
	result := r.db.Model(&models.Email{}).
		Where("created_at < ? AND (html_body <> '' OR text_body <> '')", before).
		Updates(map[string]interface{}{"html_body": "", "text_body": ""})
	return result.RowsAffected, result.Error
}

// ScrubAttachmentsOlderThan clears metadata but keeps the record; the caller deletes the blobs.
func (r *EmailRepository) ScrubAttachmentsOlderThan(before time.Time) (int64, error) {
	result := r.db.Model(&models.Email{}).
		Where("created_at < ? AND attachments_json <> ''", before).
		Update("attachments_json", "")
	return result.RowsAffected, result.Error
}
