// SPDX-FileCopyrightText: 2026 Jonas Kaninda
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

// FindByWorkspaceID returns the workspace's settings, creating a default row if
// none exists. Mirrors UserSettingRepository.FindByUserID.
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

// CreateOrUpdate saves or updates the workspace's settings row.
func (r *WorkspaceSettingRepository) CreateOrUpdate(setting *models.WorkspaceSetting) error {
	return r.db.Save(setting).Error
}

// RequireVerifiedDomain reports whether strict domain mode is enabled for the
// workspace. A focused read for the email-send hot path that does not
// auto-create a settings row; returns false when no row exists.
func (r *WorkspaceSettingRepository) RequireVerifiedDomain(workspaceID uint) bool {
	var setting models.WorkspaceSetting
	if err := r.db.Select("require_verified_domain").
		Where("workspace_id = ?", workspaceID).
		First(&setting).Error; err != nil {
		return false
	}
	return setting.RequireVerifiedDomain
}
