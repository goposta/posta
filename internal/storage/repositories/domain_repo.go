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
	"strings"

	"github.com/goposta/posta/internal/models"
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

	r.db.Model(&models.Domain{}).Where("user_id = ? AND workspace_id IS NULL", userID).Count(&total)

	if err := r.db.Where("user_id = ? AND workspace_id IS NULL", userID).
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

func (r *DomainRepository) FindByWorkspaceID(workspaceID uint, limit, offset int) ([]models.Domain, int64, error) {
	var domains []models.Domain
	var total int64

	r.db.Model(&models.Domain{}).Where("workspace_id = ?", workspaceID).Count(&total)

	if err := r.db.Where("workspace_id = ?", workspaceID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&domains).Error; err != nil {
		return nil, 0, err
	}
	return domains, total, nil
}

func (r *DomainRepository) FindByScope(scope ResourceScope, limit, offset int) ([]models.Domain, int64, error) {
	var items []models.Domain
	var total int64

	ApplyScope(r.db.Model(&models.Domain{}), scope).Count(&total)

	if err := ApplyScope(r.db, scope).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

type AdminDomainFilter struct {
	Search      string
	Verified    *bool
	WorkspaceID *uint
}

func (r *DomainRepository) FindAllFiltered(f AdminDomainFilter, limit, offset int) ([]models.Domain, int64, error) {
	var items []models.Domain
	var total int64

	apply := func(q *gorm.DB) *gorm.DB {
		if s := strings.TrimSpace(f.Search); s != "" {
			q = q.Where("LOWER(domain) LIKE ?", "%"+strings.ToLower(s)+"%")
		}
		if f.Verified != nil {
			q = q.Where("ownership_verified = ?", *f.Verified)
		}
		if f.WorkspaceID != nil {
			q = q.Where("workspace_id = ?", *f.WorkspaceID)
		}
		return q
	}

	apply(r.db.Model(&models.Domain{})).Count(&total)

	if err := apply(r.db).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *DomainRepository) CountAll() (total, verified int64) {
	r.db.Model(&models.Domain{}).Count(&total)
	r.db.Model(&models.Domain{}).Where("ownership_verified = ?", true).Count(&verified)
	return
}

func (r *DomainRepository) FindVerifiedByName(domainName string) (*models.Domain, error) {
	var d models.Domain
	if err := r.db.Where("LOWER(domain) = LOWER(?) AND ownership_verified = ?", domainName, true).
		Order("created_at ASC").
		First(&d).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *DomainRepository) IsOwnershipVerified(userID uint, domainName string) bool {
	var count int64
	r.db.Model(&models.Domain{}).
		Where("user_id = ? AND domain = ? AND ownership_verified = ?", userID, domainName, true).
		Count(&count)
	return count > 0
}

// IsOwnershipVerifiedInWorkspace accepts a domain verified by any member of the workspace.
func (r *DomainRepository) IsOwnershipVerifiedInWorkspace(workspaceID uint, domainName string) bool {
	var count int64
	r.db.Model(&models.Domain{}).
		Where("workspace_id = ? AND domain = ? AND ownership_verified = ?", workspaceID, domainName, true).
		Count(&count)
	return count > 0
}
