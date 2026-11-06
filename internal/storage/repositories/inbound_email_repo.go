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
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
)

type InboundFilter struct {
	Status string
	Source string
	Sender string
	Query  string
}

type InboundEmailRepository struct {
	db *gorm.DB
}

func NewInboundEmailRepository(db *gorm.DB) *InboundEmailRepository {
	return &InboundEmailRepository{db: db}
}

func (r *InboundEmailRepository) Create(em *models.InboundEmail) error {
	return r.db.Create(em).Error
}

func (r *InboundEmailRepository) Update(em *models.InboundEmail) error {
	return r.db.Save(em).Error
}

func (r *InboundEmailRepository) FindByID(id uint) (*models.InboundEmail, error) {
	var em models.InboundEmail
	if err := r.db.First(&em, id).Error; err != nil {
		return nil, err
	}
	return &em, nil
}

func (r *InboundEmailRepository) FindByUUID(uuid string) (*models.InboundEmail, error) {
	var em models.InboundEmail
	if err := r.db.Where("uuid = ?", uuid).First(&em).Error; err != nil {
		return nil, err
	}
	return &em, nil
}

func (r *InboundEmailRepository) FindByUserID(userID uint, limit, offset int) ([]models.InboundEmail, int64, error) {
	var items []models.InboundEmail
	var total int64

	r.db.Model(&models.InboundEmail{}).Where("user_id = ? AND workspace_id IS NULL", userID).Count(&total)

	if err := r.db.Where("user_id = ? AND workspace_id IS NULL", userID).
		Order("received_at DESC").
		Limit(limit).Offset(offset).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *InboundEmailRepository) FindByWorkspaceID(workspaceID uint, limit, offset int) ([]models.InboundEmail, int64, error) {
	var items []models.InboundEmail
	var total int64

	r.db.Model(&models.InboundEmail{}).Where("workspace_id = ?", workspaceID).Count(&total)

	if err := r.db.Where("workspace_id = ?", workspaceID).
		Order("received_at DESC").
		Limit(limit).Offset(offset).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *InboundEmailRepository) FindByScope(scope ResourceScope, limit, offset int) ([]models.InboundEmail, int64, error) {
	var items []models.InboundEmail
	var total int64

	ApplyScope(r.db.Model(&models.InboundEmail{}), scope).Count(&total)

	if err := ApplyScope(r.db, scope).
		Order("received_at DESC").
		Limit(limit).Offset(offset).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// FindByMessageID backs idempotent ingest, since MX providers may retry webhooks.
func (r *InboundEmailRepository) FindByMessageID(userID uint, messageID string) (*models.InboundEmail, error) {
	var em models.InboundEmail
	if err := r.db.Where("user_id = ? AND message_id = ?", userID, messageID).First(&em).Error; err != nil {
		return nil, err
	}
	return &em, nil
}

// FindByDedupHash is the dedup fallback when Message-ID is absent (webhook ingest).
func (r *InboundEmailRepository) FindByDedupHash(userID uint, hash string) (*models.InboundEmail, error) {
	var em models.InboundEmail
	if err := r.db.Where("user_id = ? AND dedup_hash = ?", userID, hash).First(&em).Error; err != nil {
		return nil, err
	}
	return &em, nil
}

// FindByScopeFiltered treats an empty filter field as no filter on that column.
func (r *InboundEmailRepository) FindByScopeFiltered(scope ResourceScope, f InboundFilter, limit, offset int) ([]models.InboundEmail, int64, error) {
	var items []models.InboundEmail
	var total int64

	apply := func(q *gorm.DB) *gorm.DB {
		q = ApplyScope(q, scope)
		if f.Status != "" {
			q = q.Where("status = ?", f.Status)
		}
		if f.Source != "" {
			q = q.Where("source = ?", f.Source)
		}
		if f.Sender != "" {
			q = q.Where("LOWER(sender) LIKE ?", "%"+strings.ToLower(f.Sender)+"%")
		}
		if f.Query != "" {
			q = q.Where("LOWER(subject) LIKE ?", "%"+strings.ToLower(f.Query)+"%")
		}
		return q
	}

	apply(r.db.Model(&models.InboundEmail{})).Count(&total)

	if err := apply(r.db).
		Order("received_at DESC").
		Limit(limit).Offset(offset).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *InboundEmailRepository) Delete(id uint) error {
	return r.db.Delete(&models.InboundEmail{}, id).Error
}

func (r *InboundEmailRepository) CountByScope(scope ResourceScope) (total, forwarded, failed, received int64, err error) {
	err = ApplyScope(r.db.Model(&models.InboundEmail{}), scope).Count(&total).Error
	if err != nil {
		return
	}
	_ = ApplyScope(r.db.Model(&models.InboundEmail{}), scope).Where("status = ?", models.InboundStatusForwarded).Count(&forwarded).Error
	_ = ApplyScope(r.db.Model(&models.InboundEmail{}), scope).Where("status = ?", models.InboundStatusFailed).Count(&failed).Error
	_ = ApplyScope(r.db.Model(&models.InboundEmail{}), scope).Where("status = ?", models.InboundStatusReceived).Count(&received).Error
	return
}

func (r *InboundEmailRepository) DeleteOlderThan(before time.Time) (int64, error) {
	result := r.db.Where("created_at < ?", before).Delete(&models.InboundEmail{})
	return result.RowsAffected, result.Error
}

// InboundBlobKeysOlderThan lets the caller remove blob objects before the rows are dropped.
func (r *InboundEmailRepository) InboundBlobKeysOlderThan(before time.Time) (attachmentsJSON []string, rawKeys []string, err error) {
	err = r.db.Model(&models.InboundEmail{}).
		Where("created_at < ? AND attachments_json <> ''", before).
		Pluck("attachments_json", &attachmentsJSON).Error
	if err != nil {
		return nil, nil, err
	}
	err = r.db.Model(&models.InboundEmail{}).
		Where("created_at < ? AND raw_storage_key <> ''", before).
		Pluck("raw_storage_key", &rawKeys).Error
	return attachmentsJSON, rawKeys, err
}

// ScrubBodiesOlderThan keeps the record; the raw .eml is scrubbed separately by ScrubRawOlderThan.
func (r *InboundEmailRepository) ScrubBodiesOlderThan(before time.Time) (int64, error) {
	result := r.db.Model(&models.InboundEmail{}).
		Where("created_at < ? AND (html_body <> '' OR text_body <> '')", before).
		Updates(map[string]interface{}{"html_body": "", "text_body": ""})
	return result.RowsAffected, result.Error
}

// ScrubRawOlderThan runs at the shorter content window, since the raw .eml holds both body and attachments.
func (r *InboundEmailRepository) ScrubRawOlderThan(before time.Time) (int64, error) {
	result := r.db.Model(&models.InboundEmail{}).
		Where("created_at < ? AND raw_storage_key <> ''", before).
		Update("raw_storage_key", "")
	return result.RowsAffected, result.Error
}

// ScrubAttachmentsOlderThan keeps the record; the caller deletes the blobs.
func (r *InboundEmailRepository) ScrubAttachmentsOlderThan(before time.Time) (int64, error) {
	result := r.db.Model(&models.InboundEmail{}).
		Where("created_at < ? AND attachments_json <> ''", before).
		Update("attachments_json", "")
	return result.RowsAffected, result.Error
}
