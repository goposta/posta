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
			UserID:             userID,
			Timezone:           "UTC",
			EmailNotifications: true,
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

// CreateOrUpdate saves or updates the user's settings row.
func (r *UserSettingRepository) CreateOrUpdate(setting *models.UserSetting) error {
	return r.db.Save(setting).Error
}

// FindUsersWithDailyReport returns user IDs that have daily_report enabled.
func (r *UserSettingRepository) FindUsersWithDailyReport() ([]uint, error) {
	var userIDs []uint
	if err := r.db.Model(&models.UserSetting{}).
		Where("daily_report = ?", true).
		Pluck("user_id", &userIDs).Error; err != nil {
		return nil, err
	}
	return userIDs, nil
}
