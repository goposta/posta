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
	"time"

	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(user *models.User) error {
	return r.db.Create(user).Error
}

func (r *UserRepository) FindByEmail(email string) (*models.User, error) {
	var user models.User
	if err := r.db.Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) FindByID(id uint) (*models.User, error) {
	var user models.User
	if err := r.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) FindAll(limit, offset int) ([]models.User, int64, error) {
	var users []models.User
	var total int64

	r.db.Model(&models.User{}).Count(&total)

	if err := r.db.Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&users).Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

func (r *UserRepository) Update(user *models.User) error {
	return r.db.Save(user).Error
}

func (r *UserRepository) Delete(id uint) error {
	return r.db.Delete(&models.User{}, id).Error
}

// DeleteAllUserData removes all data owned by a user
func (r *UserRepository) DeleteAllUserData(userID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		tables := []string{
			"sessions",
			"webhook_deliveries",
			"bounces",
			"suppressions",
			"tracking_events",
			"tracked_links",
			"campaign_messages",
			"campaigns",
			"subscriber_list_members",
			"subscriber_lists",
			"subscribers",
			"template_localizations",
			"template_versions",
			"templates",
			"style_sheets",
			"languages",
			"contacts",
			"emails",
			"api_keys",
			"webhooks",
			"domains",
			"smtp_servers",
			"events",
			"user_settings",
			"oauth_accounts",
		}
		for _, table := range tables {
			if err := tx.Exec("DELETE FROM "+table+" WHERE user_id = ?", userID).Error; err != nil {
				return err
			}
		}

		// Remove workspace memberships (but not workspaces they don't own)
		if err := tx.Exec("DELETE FROM workspace_members WHERE user_id = ?", userID).Error; err != nil {
			return err
		}

		// Delete workspaces owned by this user (and their members/invitations)
		var ownedWSIDs []uint
		if err := tx.Raw("SELECT id FROM workspaces WHERE owner_id = ?", userID).Scan(&ownedWSIDs).Error; err != nil {
			return err
		}
		if len(ownedWSIDs) > 0 {
			if err := tx.Exec("DELETE FROM workspace_invitations WHERE workspace_id IN ?", ownedWSIDs).Error; err != nil {
				return err
			}
			if err := tx.Exec("DELETE FROM workspace_members WHERE workspace_id IN ?", ownedWSIDs).Error; err != nil {
				return err
			}
			if err := tx.Exec("DELETE FROM workspaces WHERE owner_id = ?", userID).Error; err != nil {
				return err
			}
		}

		return tx.Delete(&models.User{}, userID).Error
	})
}

// FindScheduledForDeletion returns users whose scheduled_deletion_at is in the past.
func (r *UserRepository) FindScheduledForDeletion() ([]models.User, error) {
	var users []models.User
	if err := r.db.Where("scheduled_deletion_at IS NOT NULL AND scheduled_deletion_at <= ?", time.Now()).
		Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}
