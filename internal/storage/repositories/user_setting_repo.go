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

func (r *UserSettingRepository) CreateOrUpdate(setting *models.UserSetting) error {
	return r.db.Save(setting).Error
}
