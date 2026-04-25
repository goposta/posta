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
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
)

// InboundFilter captures optional list filters for inbound email queries.
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

// FindByMessageID returns an existing inbound email with the same Message-ID for
// the given user. Used for idempotent ingest (MX providers may retry webhooks).
// Returns gorm.ErrRecordNotFound when no match exists.
func (r *InboundEmailRepository) FindByMessageID(userID uint, messageID string) (*models.InboundEmail, error) {
	var em models.InboundEmail
	if err := r.db.Where("user_id = ? AND message_id = ?", userID, messageID).First(&em).Error; err != nil {
		return nil, err
	}
	return &em, nil
}

// FindByDedupHash returns an existing inbound email with the same dedup fallback
// hash for the given user. Used when Message-ID is absent (webhook ingest).
func (r *InboundEmailRepository) FindByDedupHash(userID uint, hash string) (*models.InboundEmail, error) {
	var em models.InboundEmail
	if err := r.db.Where("user_id = ? AND dedup_hash = ?", userID, hash).First(&em).Error; err != nil {
		return nil, err
	}
	return &em, nil
}

// FindByScopeFiltered returns inbound emails matching the scope and any optional
// filters. An empty filter field means "no filter on that column".
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

// Delete permanently removes an inbound email record by ID.
func (r *InboundEmailRepository) Delete(id uint) error {
	return r.db.Delete(&models.InboundEmail{}, id).Error
}

// CountByScope returns aggregate counts per status for dashboard use.
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

// DeleteOlderThan removes inbound email records older than the given time.
// Returns the number of rows deleted.
func (r *InboundEmailRepository) DeleteOlderThan(before time.Time) (int64, error) {
	result := r.db.Where("created_at < ?", before).Delete(&models.InboundEmail{})
	return result.RowsAffected, result.Error
}

// InboundBlobKeysOlderThan returns the attachments_json and raw_storage_key
// values for every inbound_emails row older than the cutoff so the caller can
// remove the underlying blob objects before the DB rows are dropped.
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
