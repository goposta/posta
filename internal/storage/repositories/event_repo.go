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
	"time"

	"github.com/jkaninda/posta/internal/models"
	"gorm.io/gorm"
)

type EventRepository struct {
	db *gorm.DB
}

func NewEventRepository(db *gorm.DB) *EventRepository {
	return &EventRepository{db: db}
}

func (r *EventRepository) Create(event *models.Event) error {
	return r.db.Create(event).Error
}

func (r *EventRepository) FindAll(limit, offset int) ([]models.Event, int64, error) {
	var events []models.Event
	var total int64

	r.db.Model(&models.Event{}).Count(&total)

	if err := r.db.Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&events).Error; err != nil {
		return nil, 0, err
	}
	return events, total, nil
}

func (r *EventRepository) FindByActorAndCategory(actorID uint, category models.EventCategory, limit, offset int) ([]models.Event, int64, error) {
	var events []models.Event
	var total int64

	r.db.Model(&models.Event{}).Where("actor_id = ? AND category = ?", actorID, category).Count(&total)

	if err := r.db.Where("actor_id = ? AND category = ?", actorID, category).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&events).Error; err != nil {
		return nil, 0, err
	}
	return events, total, nil
}

func (r *EventRepository) FindByCategory(category models.EventCategory, limit, offset int) ([]models.Event, int64, error) {
	var events []models.Event
	var total int64

	r.db.Model(&models.Event{}).Where("category = ?", category).Count(&total)

	if err := r.db.Where("category = ?", category).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&events).Error; err != nil {
		return nil, 0, err
	}
	return events, total, nil
}

// DeleteOlderThan deletes event records older than the given time.
// Returns the number of rows deleted.
func (r *EventRepository) DeleteOlderThan(before time.Time) (int64, error) {
	result := r.db.Where("created_at < ?", before).Delete(&models.Event{})
	return result.RowsAffected, result.Error
}
