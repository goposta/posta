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

type WorkspaceSettingRepository struct {
	db *gorm.DB
}

func NewWorkspaceSettingRepository(db *gorm.DB) *WorkspaceSettingRepository {
	return &WorkspaceSettingRepository{db: db}
}

// FindByWorkspaceID creates a default settings row if none exists.
func (r *WorkspaceSettingRepository) FindByWorkspaceID(workspaceID uint) (*models.WorkspaceSetting, error) {
	var setting models.WorkspaceSetting
	result := r.db.Where("workspace_id = ?", workspaceID).First(&setting)
	if result.Error == nil {
		return &setting, nil
	}
	if result.Error == gorm.ErrRecordNotFound {
		setting = models.WorkspaceSetting{
			WorkspaceID:        workspaceID,
			Timezone:           "UTC",
			WebhookRetryCount:  3,
			APIKeyExpiryDays:   90,
			BounceAutoSuppress: true,
		}
		if err := r.db.Create(&setting).Error; err != nil {
			return nil, err
		}
		return &setting, nil
	}
	return nil, result.Error
}

func (r *WorkspaceSettingRepository) CreateOrUpdate(setting *models.WorkspaceSetting) error {
	return r.db.Save(setting).Error
}

// RequireVerifiedDomain is a hot-path read that never creates a settings row; false when none exists.
func (r *WorkspaceSettingRepository) RequireVerifiedDomain(workspaceID uint) bool {
	var setting models.WorkspaceSetting
	if err := r.db.Select("require_verified_domain").
		Where("workspace_id = ?", workspaceID).
		First(&setting).Error; err != nil {
		return false
	}
	return setting.RequireVerifiedDomain
}
