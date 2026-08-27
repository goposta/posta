// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package repositories

import (
	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
)

type UserSettingRepository struct {
	db *gorm.DB
}

func NewUserSettingRepository(db *gorm.DB) *UserSettingRepository {
	return &UserSettingRepository{db: db}
}

// FindByUserID returns the user's settings, creating a default row if none exists.
func (r *UserSettingRepository) FindByUserID(userID uint) (*models.UserSetting, error) {
	var setting models.UserSetting
	result := r.db.Where("user_id = ?", userID).First(&setting)
	if result.Error == nil {
		return &setting, nil
	}
	if result.Error == gorm.ErrRecordNotFound {
		setting = models.UserSetting{
			UserID:                  userID,
			Timezone:                "UTC",
			EmailNotifications:      true,
			WebhookRetryCount:       3,
			APIKeyExpiryDays:        90,
			BounceAutoSuppress:      true,
			DailyReport:             true,
			NotifyBounceAlerts:      true,
			NotifyAPIKeyExpiry:      true,
			NotifyWorkspaceActivity: true,
		}
		if err := r.db.Create(&setting).Error; err != nil {
			return nil, err
		}
		return &setting, nil
	}
	return nil, result.Error
}

// CreateOrUpdate saves or updates the user's settings row.
func (r *UserSettingRepository) CreateOrUpdate(setting *models.UserSetting) error {
	return r.db.Save(setting).Error
}
