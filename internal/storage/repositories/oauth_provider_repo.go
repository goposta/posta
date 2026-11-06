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

type OAuthProviderRepository struct {
	db *gorm.DB
}

func NewOAuthProviderRepository(db *gorm.DB) *OAuthProviderRepository {
	return &OAuthProviderRepository{db: db}
}

// Count counts the configured providers, enabled or not.
func (r *OAuthProviderRepository) Count() (int64, error) {
	var n int64
	err := r.db.Model(&models.OAuthProvider{}).Count(&n).Error
	return n, err
}

func (r *OAuthProviderRepository) Create(p *models.OAuthProvider) error {
	return r.db.Create(p).Error
}

func (r *OAuthProviderRepository) FindByID(id uint) (*models.OAuthProvider, error) {
	var p models.OAuthProvider
	if err := r.db.First(&p, id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *OAuthProviderRepository) FindByName(name string) (*models.OAuthProvider, error) {
	var p models.OAuthProvider
	if err := r.db.Where("name = ? AND enabled = true", name).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// FindEnabled returns enabled, non-hidden global providers for the login page button list.
func (r *OAuthProviderRepository) FindEnabled() ([]models.OAuthProvider, error) {
	var providers []models.OAuthProvider
	if err := r.db.Where("enabled = true AND hidden = false AND workspace_id IS NULL").
		Order("display_name ASC").Find(&providers).Error; err != nil {
		return nil, err
	}
	return providers, nil
}

// FindEnabledForWorkspace returns enabled, non-hidden providers: global + workspace-scoped.
func (r *OAuthProviderRepository) FindEnabledForWorkspace(wsID uint) ([]models.OAuthProvider, error) {
	var providers []models.OAuthProvider
	if err := r.db.Where("enabled = true AND hidden = false AND (workspace_id IS NULL OR workspace_id = ?)", wsID).
		Order("display_name ASC").Find(&providers).Error; err != nil {
		return nil, err
	}
	return providers, nil
}

// HasEnabledHidden drives whether the login page shows the "Continue with SSO" entry point.
func (r *OAuthProviderRepository) HasEnabledHidden() (bool, error) {
	var count int64
	if err := r.db.Model(&models.OAuthProvider{}).
		Where("enabled = true AND hidden = true").Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// FindEnabledByDomain includes hidden providers, matching their AllowedDomains CSV, for email-based SSO discovery.
func (r *OAuthProviderRepository) FindEnabledByDomain(domain string) (*models.OAuthProvider, error) {
	if domain == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var providers []models.OAuthProvider
	if err := r.db.Where("enabled = true AND allowed_domains <> ''").
		Find(&providers).Error; err != nil {
		return nil, err
	}
	target := strings.ToLower(strings.TrimSpace(domain))
	for i := range providers {
		for _, d := range strings.Split(providers[i].AllowedDomains, ",") {
			if strings.ToLower(strings.TrimSpace(d)) == target {
				return &providers[i], nil
			}
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *OAuthProviderRepository) FindAll() ([]models.OAuthProvider, error) {
	var providers []models.OAuthProvider
	if err := r.db.Order("created_at DESC").Find(&providers).Error; err != nil {
		return nil, err
	}
	return providers, nil
}

func (r *OAuthProviderRepository) Update(p *models.OAuthProvider) error {
	return r.db.Save(p).Error
}

func (r *OAuthProviderRepository) Delete(id uint) error {
	return r.db.Delete(&models.OAuthProvider{}, id).Error
}
