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
	"fmt"
	"strings"

	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SubscriberRepository struct {
	db *gorm.DB
}

func NewSubscriberRepository(db *gorm.DB) *SubscriberRepository {
	return &SubscriberRepository{db: db}
}

func (r *SubscriberRepository) Create(s *models.Subscriber) error {
	return r.db.Create(s).Error
}

func (r *SubscriberRepository) FindByID(id uint) (*models.Subscriber, error) {
	var s models.Subscriber
	if err := r.db.First(&s, id).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *SubscriberRepository) FindByEmail(scope ResourceScope, email string) (*models.Subscriber, error) {
	var s models.Subscriber
	q := ApplyScope(r.db, scope).Where("email = ?", strings.ToLower(strings.TrimSpace(email)))
	if err := q.First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

// FindAllByEmail finds all subscribers with the given email address across all scopes.
func (r *SubscriberRepository) FindAllByEmail(email string, result *[]models.Subscriber) error {
	return r.db.Where("email = ?", strings.ToLower(strings.TrimSpace(email))).Find(result).Error
}

func (r *SubscriberRepository) FindByScope(scope ResourceScope, search, status string, limit, offset int) ([]models.Subscriber, int64, error) {
	var items []models.Subscriber
	var total int64

	q := ApplyScope(r.db.Model(&models.Subscriber{}), scope)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if search != "" {
		q = q.Where("email ILIKE ? OR name ILIKE ?", "%"+search+"%", "%"+search+"%")
	}
	q.Count(&total)

	qFind := ApplyScope(r.db, scope)
	if status != "" {
		qFind = qFind.Where("status = ?", status)
	}
	if search != "" {
		qFind = qFind.Where("email ILIKE ? OR name ILIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if err := qFind.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *SubscriberRepository) Update(s *models.Subscriber) error {
	return r.db.Save(s).Error
}

func (r *SubscriberRepository) Delete(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("subscriber_id = ?", id).Delete(&models.SubscriberListMember{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.Subscriber{}, id).Error
	})
}

// BulkCreate inserts subscribers in batches, skipping duplicates.
func (r *SubscriberRepository) BulkCreate(subscribers []models.Subscriber) (created int, skipped int, err error) {
	if len(subscribers) == 0 {
		return 0, 0, nil
	}
	result := r.db.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(subscribers, 100)
	if result.Error != nil {
		return 0, 0, result.Error
	}
	created = int(result.RowsAffected)
	skipped = len(subscribers) - created
	return created, skipped, nil
}

// FindByFilterRules evaluates dynamic segment filter rules against subscribers.
func (r *SubscriberRepository) FindByFilterRules(scope ResourceScope, rules models.FilterRules, limit, offset int) ([]models.Subscriber, int64, error) {
	var items []models.Subscriber
	var total int64

	q := r.applyFilterRules(ApplyScope(r.db.Model(&models.Subscriber{}), scope), rules)
	q.Count(&total)

	qFind := r.applyFilterRules(ApplyScope(r.db, scope), rules)
	if err := qFind.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *SubscriberRepository) CountByFilterRules(scope ResourceScope, rules models.FilterRules) (int64, error) {
	var count int64
	q := r.applyFilterRules(ApplyScope(r.db.Model(&models.Subscriber{}), scope), rules)
	if err := q.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *SubscriberRepository) applyFilterRules(q *gorm.DB, rules models.FilterRules) *gorm.DB {
	for _, rule := range rules {
		field := rule.Field
		op := rule.Operator
		val := rule.Value

		if strings.HasPrefix(field, "engagement.") {
			engagementField := strings.TrimPrefix(field, "engagement.")
			switch engagementField {
			case "opened_campaign":
				q = q.Where("EXISTS (SELECT 1 FROM campaign_messages WHERE campaign_messages.subscriber_id = subscribers.id AND campaign_messages.campaign_id = ? AND campaign_messages.opened_at IS NOT NULL)", val)
			case "clicked_campaign":
				q = q.Where("EXISTS (SELECT 1 FROM campaign_messages WHERE campaign_messages.subscriber_id = subscribers.id AND campaign_messages.campaign_id = ? AND campaign_messages.clicked_at IS NOT NULL)", val)
			case "not_opened_campaign":
				q = q.Where("NOT EXISTS (SELECT 1 FROM campaign_messages WHERE campaign_messages.subscriber_id = subscribers.id AND campaign_messages.campaign_id = ? AND campaign_messages.opened_at IS NOT NULL)", val)
			case "received_campaign":
				q = q.Where("EXISTS (SELECT 1 FROM campaign_messages WHERE campaign_messages.subscriber_id = subscribers.id AND campaign_messages.campaign_id = ?)", val)
			case "inactive_days":
				q = q.Where("NOT EXISTS (SELECT 1 FROM campaign_messages WHERE campaign_messages.subscriber_id = subscribers.id AND campaign_messages.opened_at > NOW() - INTERVAL '1 day' * ?)", val)
			case "active_days":
				q = q.Where("EXISTS (SELECT 1 FROM campaign_messages WHERE campaign_messages.subscriber_id = subscribers.id AND campaign_messages.opened_at > NOW() - INTERVAL '1 day' * ?)", val)
			case "total_opens_gte":
				q = q.Where("(SELECT COUNT(*) FROM campaign_messages WHERE campaign_messages.subscriber_id = subscribers.id AND campaign_messages.opened_at IS NOT NULL) >= ?", val)
			case "total_clicks_gte":
				q = q.Where("(SELECT COUNT(*) FROM campaign_messages WHERE campaign_messages.subscriber_id = subscribers.id AND campaign_messages.clicked_at IS NOT NULL) >= ?", val)
			}
			continue
		}

		var column string
		if strings.HasPrefix(field, "custom_fields.") {
			// The JSON key is sanitized because it is interpolated into SQL.
			jsonKey := strings.TrimPrefix(field, "custom_fields.")
			if !isAlphanumericUnderscore(jsonKey) || jsonKey == "" {
				continue
			}
			column = fmt.Sprintf("custom_fields::jsonb->>'%s'", jsonKey)
		} else {
			// Only known columns, so user input never names one.
			switch field {
			case "email", "name", "status":
				column = field
			default:
				continue
			}
		}

		switch op {
		case "eq":
			q = q.Where(fmt.Sprintf("%s = ?", column), val)
		case "neq":
			q = q.Where(fmt.Sprintf("%s != ?", column), val)
		case "contains":
			q = q.Where(fmt.Sprintf("%s ILIKE ?", column), fmt.Sprintf("%%%v%%", val))
		case "starts_with":
			q = q.Where(fmt.Sprintf("%s ILIKE ?", column), fmt.Sprintf("%v%%", val))
		case "ends_with":
			q = q.Where(fmt.Sprintf("%s ILIKE ?", column), fmt.Sprintf("%%%v", val))
		case "gt":
			q = q.Where(fmt.Sprintf("%s > ?", column), val)
		case "lt":
			q = q.Where(fmt.Sprintf("%s < ?", column), val)
		case "in":
			q = q.Where(fmt.Sprintf("%s IN ?", column), val)
		}
	}
	return q
}

// isAlphanumericUnderscore returns true if s contains only [a-zA-Z0-9_-].
func isAlphanumericUnderscore(s string) bool {
	for _, c := range s {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '_' && c != '-' {
			return false
		}
	}
	return true
}
