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
	"time"

	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
)

type ContactRepository struct {
	db *gorm.DB
}

func NewContactRepository(db *gorm.DB) *ContactRepository {
	return &ContactRepository{db: db}
}

// RecordSent upserts contacts for the given recipients, incrementing sent_count.
// Recipients may use RFC 5322 format ("Display Name <email>"), in which case
// the name and bare email are extracted.
func (r *ContactRepository) RecordSent(userID uint, workspaceID *uint, recipients []string) {
	now := time.Now()
	for _, raw := range recipients {
		addr, name := parseRecipient(raw)
		r.db.Exec(`INSERT INTO contacts (user_id, workspace_id, email, name, sent_count, fail_count, last_sent_at, created_at)
			VALUES (?, ?, ?, ?, 1, 0, ?, ?)
			ON CONFLICT (user_id, COALESCE(workspace_id, 0), email)
			DO UPDATE SET sent_count = contacts.sent_count + 1, last_sent_at = EXCLUDED.last_sent_at`+
			nameUpdateClause(name),
			userID, workspaceID, addr, name, now, now)
	}
}

// RecordFailed upserts contacts for the given recipients, incrementing fail_count.
func (r *ContactRepository) RecordFailed(userID uint, workspaceID *uint, recipients []string) {
	for _, raw := range recipients {
		addr, name := parseRecipient(raw)
		r.db.Exec(`INSERT INTO contacts (user_id, workspace_id, email, name, sent_count, fail_count, created_at)
			VALUES (?, ?, ?, ?, 0, 1, ?)
			ON CONFLICT (user_id, COALESCE(workspace_id, 0), email)
			DO UPDATE SET fail_count = contacts.fail_count + 1`+
			nameUpdateClause(name),
			userID, workspaceID, addr, name, time.Now())
	}
}

func nameUpdateClause(name string) string {
	if name != "" {
		return `, name = EXCLUDED.name`
	}
	return ""
}

// parseRecipient extracts the bare email and display name from a recipient
// string. It supports RFC 5322 format ("Display Name <email@example.com>").
// If parsing fails, the original string is returned as the email with no name.
func parseRecipient(raw string) (email, name string) {
	addr, err := mail.ParseAddress(raw)
	if err != nil {
		return raw, ""
	}
	return addr.Address, addr.Name
}

func (r *ContactRepository) FindByUserID(userID uint, search string, limit, offset int) ([]models.Contact, int64, error) {
	var contacts []models.Contact
	var total int64

	q := r.db.Model(&models.Contact{}).Where("user_id = ? AND workspace_id IS NULL", userID)
	if search != "" {
		q = q.Where("email ILIKE ? OR name ILIKE ?", "%"+search+"%", "%"+search+"%")
	}
	q.Count(&total)

	if err := q.Order("last_sent_at DESC NULLS LAST").
		Limit(limit).Offset(offset).
		Find(&contacts).Error; err != nil {
		return nil, 0, err
	}
	return contacts, total, nil
}

func (r *ContactRepository) FindByWorkspaceID(workspaceID uint, search string, limit, offset int) ([]models.Contact, int64, error) {
	var contacts []models.Contact
	var total int64

	q := r.db.Model(&models.Contact{}).Where("workspace_id = ?", workspaceID)
	if search != "" {
		q = q.Where("email ILIKE ? OR name ILIKE ?", "%"+search+"%", "%"+search+"%")
	}
	q.Count(&total)

	if err := q.Order("last_sent_at DESC NULLS LAST").
		Limit(limit).Offset(offset).
		Find(&contacts).Error; err != nil {
		return nil, 0, err
	}
	return contacts, total, nil
}

func (r *ContactRepository) FindByScope(scope ResourceScope, search string, limit, offset int) ([]models.Contact, int64, error) {
	var contacts []models.Contact
	var total int64

	q := ApplyScope(r.db.Model(&models.Contact{}), scope)
	if search != "" {
		q = q.Where("email ILIKE ? OR name ILIKE ?", "%"+search+"%", "%"+search+"%")
	}
	q.Count(&total)

	qFind := ApplyScope(r.db, scope)
	if search != "" {
		qFind = qFind.Where("email ILIKE ? OR name ILIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if err := qFind.Order("last_sent_at DESC NULLS LAST").
		Limit(limit).Offset(offset).
		Find(&contacts).Error; err != nil {
		return nil, 0, err
	}
	return contacts, total, nil
}

func (r *ContactRepository) FindByID(id uint) (*models.Contact, error) {
	var contact models.Contact
	if err := r.db.First(&contact, id).Error; err != nil {
		return nil, err
	}
	return &contact, nil
}
