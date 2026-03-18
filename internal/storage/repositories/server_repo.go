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
	"github.com/jkaninda/posta/internal/models"
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

func (r *ServerRepository) FindAll(limit, offset int) ([]models.Server, int64, error) {
	var servers []models.Server
	var total int64

	r.db.Model(&models.Server{}).Count(&total)

	if err := r.db.Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&servers).Error; err != nil {
		return nil, 0, err
	}
	return servers, total, nil
}

// FindEnabledByDomain returns the first enabled shared server whose allowed_domains
// contains the given domain.
func (r *ServerRepository) FindEnabledByDomain(domain string) (*models.Server, error) {
	var server models.Server
	if err := r.db.
		Where("status = ? AND ? = ANY(allowed_domains)", models.SMTPStatusEnabled, domain).
		First(&server).Error; err != nil {
		return nil, err
	}
	return &server, nil
}

// IncrementSentCount atomically increases the sent_count by 1 for the given server.
func (r *ServerRepository) IncrementSentCount(id uint) {
	r.db.Exec("UPDATE servers SET sent_count = sent_count + 1 WHERE id = ?", id)
}

// IncrementFailedCount atomically increases the failed_count by 1 for the given server.
func (r *ServerRepository) IncrementFailedCount(id uint) {
	r.db.Exec("UPDATE servers SET failed_count = failed_count + 1 WHERE id = ?", id)
}

// SetStatus updates the status, validation error, and validated_at timestamp for a server.
func (r *ServerRepository) SetStatus(id uint, status, validationError string) error {
	return r.db.Model(&models.Server{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":           status,
		"validation_error": validationError,
		"validated_at":     gorm.Expr("NOW()"),
	}).Error
}
