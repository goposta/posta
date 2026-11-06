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

type SMTPRepository struct {
	db *gorm.DB
}

func NewSMTPRepository(db *gorm.DB) *SMTPRepository {
	return &SMTPRepository{db: db}
}

func (r *SMTPRepository) Create(server *models.SMTPServer) error {
	return r.db.Create(server).Error
}

func (r *SMTPRepository) Update(server *models.SMTPServer) error {
	return r.db.Save(server).Error
}

func (r *SMTPRepository) Delete(id uint) error {
	return r.db.Delete(&models.SMTPServer{}, id).Error
}

func (r *SMTPRepository) FindByID(id uint) (*models.SMTPServer, error) {
	var server models.SMTPServer
	if err := r.db.First(&server, id).Error; err != nil {
		return nil, err
	}
	return &server, nil
}

func (r *SMTPRepository) FindByUserID(userID uint, limit, offset int) ([]models.SMTPServer, int64, error) {
	var servers []models.SMTPServer
	var total int64

	r.db.Model(&models.SMTPServer{}).Where("user_id = ? AND workspace_id IS NULL", userID).Count(&total)

	if err := r.db.Where("user_id = ? AND workspace_id IS NULL", userID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&servers).Error; err != nil {
		return nil, 0, err
	}
	return servers, total, nil
}

func (r *SMTPRepository) FindAllWithRetries() ([]models.SMTPServer, error) {
	var servers []models.SMTPServer
	if err := r.db.Where("max_retries > 0 AND status = ?", models.SMTPStatusEnabled).Find(&servers).Error; err != nil {
		return nil, err
	}
	return servers, nil
}

func (r *SMTPRepository) FindFirstByUserID(userID uint) (*models.SMTPServer, error) {
	var server models.SMTPServer
	if err := r.db.Where("user_id = ? AND status = ? AND workspace_id IS NULL", userID, models.SMTPStatusEnabled).First(&server).Error; err != nil {
		return nil, err
	}
	return &server, nil
}

func (r *SMTPRepository) FindByWorkspaceID(workspaceID uint, limit, offset int) ([]models.SMTPServer, int64, error) {
	var servers []models.SMTPServer
	var total int64

	r.db.Model(&models.SMTPServer{}).Where("workspace_id = ?", workspaceID).Count(&total)

	if err := r.db.Where("workspace_id = ?", workspaceID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&servers).Error; err != nil {
		return nil, 0, err
	}
	return servers, total, nil
}

func (r *SMTPRepository) FindByScope(scope ResourceScope, limit, offset int) ([]models.SMTPServer, int64, error) {
	var items []models.SMTPServer
	var total int64

	ApplyScope(r.db.Model(&models.SMTPServer{}), scope).Count(&total)

	if err := ApplyScope(r.db, scope).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *SMTPRepository) FindFirstByWorkspaceID(workspaceID uint) (*models.SMTPServer, error) {
	var server models.SMTPServer
	if err := r.db.Where("workspace_id = ? AND status = ?", workspaceID, models.SMTPStatusEnabled).First(&server).Error; err != nil {
		return nil, err
	}
	return &server, nil
}

// FindSystem ignores status: a dashboard toggle must not lock operators out of password resets and security alerts.
func (r *SMTPRepository) FindSystem() (*models.SMTPServer, error) {
	var server models.SMTPServer
	if err := r.db.Where("is_system = ?", true).Order("id").First(&server).Error; err != nil {
		return nil, err
	}
	return &server, nil
}

func (r *SMTPRepository) SetStatus(id uint, status, validationError string) error {
	return r.db.Model(&models.SMTPServer{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":           status,
		"validation_error": validationError,
		"validated_at":     gorm.Expr("NOW()"),
	}).Error
}
