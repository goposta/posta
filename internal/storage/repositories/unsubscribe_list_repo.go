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
	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
)

type UnsubscribeListRepository struct {
	db *gorm.DB
}

func NewUnsubscribeListRepository(db *gorm.DB) *UnsubscribeListRepository {
	return &UnsubscribeListRepository{db: db}
}

func (r *UnsubscribeListRepository) Create(list *models.UnsubscribeList) error {
	return r.db.Create(list).Error
}

func (r *UnsubscribeListRepository) FindByID(id uint) (*models.UnsubscribeList, error) {
	var list models.UnsubscribeList
	if err := r.db.First(&list, id).Error; err != nil {
		return nil, err
	}
	return &list, nil
}

// FindByIDForScope returns the list only if it belongs to scope, validating ownership at send time.
func (r *UnsubscribeListRepository) FindByIDForScope(id uint, scope ResourceScope) (*models.UnsubscribeList, error) {
	var list models.UnsubscribeList
	if err := ApplyScope(r.db, scope).First(&list, id).Error; err != nil {
		return nil, err
	}
	return &list, nil
}

func (r *UnsubscribeListRepository) FindByScope(scope ResourceScope, limit, offset int) ([]models.UnsubscribeList, int64, error) {
	var items []models.UnsubscribeList
	var total int64

	ApplyScope(r.db.Model(&models.UnsubscribeList{}), scope).Count(&total)

	if err := ApplyScope(r.db, scope).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *UnsubscribeListRepository) Update(list *models.UnsubscribeList) error {
	return r.db.Save(list).Error
}

func (r *UnsubscribeListRepository) Delete(id uint) error {
	return r.db.Delete(&models.UnsubscribeList{}, id).Error
}
