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
	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SubscriberListRepository struct {
	db *gorm.DB
}

func NewSubscriberListRepository(db *gorm.DB) *SubscriberListRepository {
	return &SubscriberListRepository{db: db}
}

func (r *SubscriberListRepository) Create(list *models.SubscriberList) error {
	return r.db.Create(list).Error
}

func (r *SubscriberListRepository) FindByID(id uint) (*models.SubscriberList, error) {
	var list models.SubscriberList
	if err := r.db.First(&list, id).Error; err != nil {
		return nil, err
	}
	return &list, nil
}

func (r *SubscriberListRepository) FindByScope(scope ResourceScope, limit, offset int) ([]models.SubscriberList, int64, error) {
	var items []models.SubscriberList
	var total int64

	ApplyScope(r.db.Model(&models.SubscriberList{}), scope).Count(&total)

	if err := ApplyScope(r.db, scope).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *SubscriberListRepository) Update(list *models.SubscriberList) error {
	return r.db.Save(list).Error
}

func (r *SubscriberListRepository) Delete(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("list_id = ?", id).Delete(&models.SubscriberListMember{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.SubscriberList{}, id).Error
	})
}

func (r *SubscriberListRepository) AddMember(member *models.SubscriberListMember) error {
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(member).Error
}

func (r *SubscriberListRepository) RemoveMember(listID, subscriberID uint) error {
	return r.db.Where("list_id = ? AND subscriber_id = ?", listID, subscriberID).
		Delete(&models.SubscriberListMember{}).Error
}

func (r *SubscriberListRepository) ListMembers(listID uint, limit, offset int) ([]models.Subscriber, int64, error) {
	var subscribers []models.Subscriber
	var total int64

	r.db.Model(&models.SubscriberListMember{}).Where("list_id = ?", listID).Count(&total)

	if err := r.db.
		Joins("JOIN subscriber_list_members ON subscriber_list_members.subscriber_id = subscribers.id").
		Where("subscriber_list_members.list_id = ?", listID).
		Order("subscribers.created_at DESC").
		Limit(limit).Offset(offset).
		Find(&subscribers).Error; err != nil {
		return nil, 0, err
	}
	return subscribers, total, nil
}

func (r *SubscriberListRepository) MemberCount(listID uint) int64 {
	var count int64
	r.db.Model(&models.SubscriberListMember{}).Where("list_id = ?", listID).Count(&count)
	return count
}

func (r *SubscriberListRepository) BulkAddMembers(listID uint, subscriberIDs []uint) (int, error) {
	if len(subscriberIDs) == 0 {
		return 0, nil
	}
	var members []models.SubscriberListMember
	for _, sid := range subscriberIDs {
		members = append(members, models.SubscriberListMember{
			ListID:       listID,
			SubscriberID: sid,
		})
	}
	result := r.db.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(members, 100)
	return int(result.RowsAffected), result.Error
}
