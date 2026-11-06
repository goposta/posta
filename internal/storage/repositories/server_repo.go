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

type ServerRepository struct {
	db *gorm.DB
}

func NewServerRepository(db *gorm.DB) *ServerRepository {
	return &ServerRepository{db: db}
}

func (r *ServerRepository) Create(server *models.Server) error {
	return r.db.Create(server).Error
}

func (r *ServerRepository) Update(server *models.Server) error {
	return r.db.Save(server).Error
}

func (r *ServerRepository) Delete(id uint) error {
	return r.db.Delete(&models.Server{}, id).Error
}

func (r *ServerRepository) FindByID(id uint) (*models.Server, error) {
	var server models.Server
	if err := r.db.First(&server, id).Error; err != nil {
		return nil, err
	}
	return &server, nil
}

func (r *ServerRepository) FindAll(search string, limit, offset int) ([]models.Server, int64, error) {
	var servers []models.Server
	var total int64

	countQ := r.db.Model(&models.Server{})
	findQ := r.db.Model(&models.Server{})
	if search != "" {
		like := "%" + search + "%"
		countQ = countQ.Where("name ILIKE ? OR host ILIKE ?", like, like)
		findQ = findQ.Where("name ILIKE ? OR host ILIKE ?", like, like)
	}

	countQ.Count(&total)

	if err := findQ.Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&servers).Error; err != nil {
		return nil, 0, err
	}
	return servers, total, nil
}

// FindEnabledByDomain returns the first enabled shared server whose allowed_domains contains domain.
func (r *ServerRepository) FindEnabledByDomain(domain string) (*models.Server, error) {
	var server models.Server
	if err := r.db.
		Where("status = ? AND ? = ANY(allowed_domains)", models.SMTPStatusEnabled, domain).
		First(&server).Error; err != nil {
		return nil, err
	}
	return &server, nil
}

func (r *ServerRepository) IncrementSentCount(id uint) {
	r.db.Exec("UPDATE servers SET sent_count = sent_count + 1 WHERE id = ?", id)
}

func (r *ServerRepository) IncrementFailedCount(id uint) {
	r.db.Exec("UPDATE servers SET failed_count = failed_count + 1 WHERE id = ?", id)
}

func (r *ServerRepository) SetStatus(id uint, status, validationError string) error {
	return r.db.Model(&models.Server{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":           status,
		"validation_error": validationError,
		"validated_at":     gorm.Expr("NOW()"),
	}).Error
}
