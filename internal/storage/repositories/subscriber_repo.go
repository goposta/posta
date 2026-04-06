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
		// Remove from all static lists first
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

// CountByFilterRules returns the count of subscribers matching filter rules.
func (r *SubscriberRepository) CountByFilterRules(scope ResourceScope, rules models.FilterRules) (int64, error) {
	var count int64
	q := r.applyFilterRules(ApplyScope(r.db.Model(&models.Subscriber{}), scope), rules)
	if err := q.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// applyFilterRules translates FilterRules into SQL WHERE clauses.
func (r *SubscriberRepository) applyFilterRules(q *gorm.DB, rules models.FilterRules) *gorm.DB {
	for _, rule := range rules {
		field := rule.Field
		op := rule.Operator
		val := rule.Value

		// Engagement-based filters — use subqueries on campaign_messages
		if strings.HasPrefix(field, "engagement.") {
			engagementField := strings.TrimPrefix(field, "engagement.")
			switch engagementField {
			case "opened_campaign":
				// Subscribers who opened a specific campaign
				q = q.Where("EXISTS (SELECT 1 FROM campaign_messages WHERE campaign_messages.subscriber_id = subscribers.id AND campaign_messages.campaign_id = ? AND campaign_messages.opened_at IS NOT NULL)", val)
			case "clicked_campaign":
				// Subscribers who clicked in a specific campaign
				q = q.Where("EXISTS (SELECT 1 FROM campaign_messages WHERE campaign_messages.subscriber_id = subscribers.id AND campaign_messages.campaign_id = ? AND campaign_messages.clicked_at IS NOT NULL)", val)
			case "not_opened_campaign":
				// Subscribers who did NOT open a specific campaign
				q = q.Where("NOT EXISTS (SELECT 1 FROM campaign_messages WHERE campaign_messages.subscriber_id = subscribers.id AND campaign_messages.campaign_id = ? AND campaign_messages.opened_at IS NOT NULL)", val)
			case "received_campaign":
				// Subscribers who received a specific campaign (regardless of open/click)
				q = q.Where("EXISTS (SELECT 1 FROM campaign_messages WHERE campaign_messages.subscriber_id = subscribers.id AND campaign_messages.campaign_id = ?)", val)
			case "inactive_days":
				// Subscribers with no opens in the last N days
				q = q.Where("NOT EXISTS (SELECT 1 FROM campaign_messages WHERE campaign_messages.subscriber_id = subscribers.id AND campaign_messages.opened_at > NOW() - INTERVAL '1 day' * ?)", val)
			case "active_days":
				// Subscribers with opens in the last N days
				q = q.Where("EXISTS (SELECT 1 FROM campaign_messages WHERE campaign_messages.subscriber_id = subscribers.id AND campaign_messages.opened_at > NOW() - INTERVAL '1 day' * ?)", val)
			case "total_opens_gte":
				// Subscribers with at least N total opens
				q = q.Where("(SELECT COUNT(*) FROM campaign_messages WHERE campaign_messages.subscriber_id = subscribers.id AND campaign_messages.opened_at IS NOT NULL) >= ?", val)
			case "total_clicks_gte":
				// Subscribers with at least N total clicks
				q = q.Where("(SELECT COUNT(*) FROM campaign_messages WHERE campaign_messages.subscriber_id = subscribers.id AND campaign_messages.clicked_at IS NOT NULL) >= ?", val)
			}
			continue
		}

		// Determine the SQL column expression
		var column string
		if strings.HasPrefix(field, "custom_fields.") {
			// JSON field extraction (PostgreSQL) - sanitize key to prevent SQL injection
			jsonKey := strings.TrimPrefix(field, "custom_fields.")
			if !isAlphanumericUnderscore(jsonKey) || jsonKey == "" {
				continue
			}
			column = fmt.Sprintf("custom_fields::jsonb->>'%s'", jsonKey)
		} else {
			// Direct column (sanitize: only allow known fields)
			switch field {
			case "email", "name", "status":
				column = field
			default:
				continue // skip unknown fields
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
