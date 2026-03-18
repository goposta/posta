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

type DomainRepository struct {
	db *gorm.DB
}

func NewDomainRepository(db *gorm.DB) *DomainRepository {
	return &DomainRepository{db: db}
}

func (r *DomainRepository) Create(domain *models.Domain) error {
	return r.db.Create(domain).Error
}

func (r *DomainRepository) Update(domain *models.Domain) error {
	return r.db.Save(domain).Error
}

func (r *DomainRepository) Delete(id uint) error {
	return r.db.Delete(&models.Domain{}, id).Error
}

func (r *DomainRepository) FindByID(id uint) (*models.Domain, error) {
	var domain models.Domain
	if err := r.db.First(&domain, id).Error; err != nil {
		return nil, err
	}
	return &domain, nil
}

func (r *DomainRepository) FindByUserID(userID uint, limit, offset int) ([]models.Domain, int64, error) {
	var domains []models.Domain
	var total int64

	r.db.Model(&models.Domain{}).Where("user_id = ?", userID).Count(&total)

	if err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&domains).Error; err != nil {
		return nil, 0, err
	}
	return domains, total, nil
}

func (r *DomainRepository) FindByUserIDAndDomain(userID uint, domain string) (*models.Domain, error) {
	var d models.Domain
	if err := r.db.Where("user_id = ? AND domain = ?", userID, domain).First(&d).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

// IsOwnershipVerified checks whether the given domain is registered and ownership-verified for the user.
func (r *DomainRepository) IsOwnershipVerified(userID uint, domainName string) bool {
	var count int64
	r.db.Model(&models.Domain{}).
		Where("user_id = ? AND domain = ? AND ownership_verified = ?", userID, domainName, true).
		Count(&count)
	return count > 0
}
