// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package repositories

import (
	"errors"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/jkaninda/logger"
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

// DefaultWorkspace resolves where a request that names no workspace should land.
// It verifies membership, so a stale default (the user left, or the workspace was
// deleted) falls back to the oldest remaining membership and is repaired in place.
// Returns (nil, "", nil) when the user belongs to no workspace at all.
func (r *UserRepository) DefaultWorkspace(userID uint) (*models.Workspace, models.WorkspaceRole, error) {
	var user models.User
	if err := r.db.Select("id", "default_workspace_id").First(&user, userID).Error; err != nil {
		return nil, "", err
	}

	if user.DefaultWorkspaceID != nil {
		ws, role, err := r.WorkspaceForMember(userID, *user.DefaultWorkspaceID)
		if err == nil {
			return ws, role, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, "", err
		}
	}

	ws, role, err := r.oldestMembership(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if user.DefaultWorkspaceID != nil {
				_ = r.SetDefaultWorkspace(userID, nil)
			}
			return nil, "", nil
		}
		return nil, "", err
	}

	if err := r.SetDefaultWorkspace(userID, &ws.ID); err != nil {
		logger.Warn("failed to repair default workspace", "user_id", userID, "error", err)
	}
	return ws, role, nil
}

// SetDefaultWorkspace records where header-less requests land. Nil clears it.
func (r *UserRepository) SetDefaultWorkspace(userID uint, workspaceID *uint) error {
	return r.db.Model(&models.User{}).Where("id = ?", userID).
		Update("default_workspace_id", workspaceID).Error
}

// WorkspaceForMember returns the workspace and the caller's role in it, or
// gorm.ErrRecordNotFound when they are not a member.
func (r *UserRepository) WorkspaceForMember(userID, workspaceID uint) (*models.Workspace, models.WorkspaceRole, error) {
	var row struct {
		models.Workspace
		Role models.WorkspaceRole
	}
	err := r.db.Model(&models.Workspace{}).
		Select("workspaces.*, workspace_members.role AS role").
		Joins("JOIN workspace_members ON workspace_members.workspace_id = workspaces.id").
		Where("workspaces.id = ? AND workspace_members.user_id = ?", workspaceID, userID).
		First(&row).Error
	if err != nil {
		return nil, "", err
	}
	ws := row.Workspace
	return &ws, row.Role, nil
}

func (r *UserRepository) oldestMembership(userID uint) (*models.Workspace, models.WorkspaceRole, error) {
	var row struct {
		models.Workspace
		Role models.WorkspaceRole
	}
	err := r.db.Model(&models.Workspace{}).
		Select("workspaces.*, workspace_members.role AS role").
		Joins("JOIN workspace_members ON workspace_members.workspace_id = workspaces.id").
		Where("workspace_members.user_id = ? AND workspaces.system = ?", userID, false).
		Order("workspace_members.created_at ASC, workspaces.id ASC").
		First(&row).Error
	if err != nil {
		return nil, "", err
	}
	ws := row.Workspace
	return &ws, row.Role, nil
}

func (r *UserRepository) FindAll(search string, limit, offset int) ([]models.User, int64, error) {
	var users []models.User
	var total int64

	countQ := r.db.Model(&models.User{})
	findQ := r.db.Model(&models.User{})
	if search != "" {
		like := "%" + search + "%"
		countQ = countQ.Where("name ILIKE ? OR email ILIKE ?", like, like)
		findQ = findQ.Where("name ILIKE ? OR email ILIKE ?", like, like)
	}

	countQ.Count(&total)

	if err := findQ.Order("created_at DESC").
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
		// Delete tracking_events via campaign_messages -> campaigns
		if err := tx.Exec("DELETE FROM tracking_events WHERE campaign_message_id IN (SELECT id FROM campaign_messages WHERE campaign_id IN (SELECT id FROM campaigns WHERE user_id = ?))", userID).Error; err != nil {
			return err
		}
		// Delete tracked_links via campaigns
		if err := tx.Exec("DELETE FROM tracked_links WHERE campaign_id IN (SELECT id FROM campaigns WHERE user_id = ?)", userID).Error; err != nil {
			return err
		}
		// Delete campaign_messages via campaigns
		if err := tx.Exec("DELETE FROM campaign_messages WHERE campaign_id IN (SELECT id FROM campaigns WHERE user_id = ?)", userID).Error; err != nil {
			return err
		}
		// Delete subscriber_list_members via subscriber_lists
		if err := tx.Exec("DELETE FROM subscriber_list_members WHERE list_id IN (SELECT id FROM subscriber_lists WHERE user_id = ?)", userID).Error; err != nil {
			return err
		}
		// Delete subscriber_list_unsubscribes via subscriber_lists
		if err := tx.Exec("DELETE FROM subscriber_list_unsubscribes WHERE list_id IN (SELECT id FROM subscriber_lists WHERE user_id = ?)", userID).Error; err != nil {
			return err
		}
		// Clear active_version_id FK on templates before deleting versions
		if err := tx.Exec("UPDATE templates SET active_version_id = NULL WHERE user_id = ?", userID).Error; err != nil {
			return err
		}
		// Delete template_localizations via template_versions -> templates
		if err := tx.Exec("DELETE FROM template_localizations WHERE version_id IN (SELECT id FROM template_versions WHERE template_id IN (SELECT id FROM templates WHERE user_id = ?))", userID).Error; err != nil {
			return err
		}
		// Delete template_versions via templates
		if err := tx.Exec("DELETE FROM template_versions WHERE template_id IN (SELECT id FROM templates WHERE user_id = ?)", userID).Error; err != nil {
			return err
		}

		// Tables with a direct user_id column
		tables := []string{
			"sessions",
			"webhook_deliveries",
			"bounces",
			"suppressions",
			"campaigns",
			"subscriber_lists",
			"subscribers",
			"templates",
			"style_sheets",
			"languages",
			"contacts",
			"unsubscribe_lists",
			"emails",
			"inbound_emails",
			"api_keys",
			"webhooks",
			"domains",
			"smtp_servers",
			"user_settings",
			"user_email_verifications",
			"o_auth_accounts",
		}

		// Delete events
		if err := tx.Exec("DELETE FROM events WHERE actor_id = ?", userID).Error; err != nil {
			return err
		}

		var contactListIDs []uint
		if err := tx.Raw("SELECT id FROM contact_lists WHERE user_id = ?", userID).Scan(&contactListIDs).Error; err != nil {
			return err
		}
		if len(contactListIDs) > 0 {
			if err := tx.Exec("DELETE FROM contact_list_members WHERE list_id IN ?", contactListIDs).Error; err != nil {
				return err
			}
		}
		if err := tx.Exec("DELETE FROM contact_lists WHERE user_id = ?", userID).Error; err != nil {
			return err
		}

		// Delete webhook_deliveries that reference the user's webhooks
		if err := tx.Exec("DELETE FROM webhook_deliveries WHERE webhook_id IN (SELECT id FROM webhooks WHERE user_id = ?)", userID).Error; err != nil {
			return err
		}

		for _, table := range tables {
			if err := tx.Exec("DELETE FROM "+table+" WHERE user_id = ?", userID).Error; err != nil {
				return err
			}
		}

		// Remove workspace memberships
		if err := tx.Exec("DELETE FROM workspace_members WHERE user_id = ?", userID).Error; err != nil {
			return err
		}

		// Delete workspaces owned
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
