// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package workspace

import (
	"errors"
	"fmt"

	"github.com/goposta/posta/internal/models"
	"github.com/jkaninda/logger"
	"gorm.io/gorm"
)

const (
	SystemSlug        = "system"
	SystemName        = "Posta System"
	SystemDescription = "Built-in workspace for platform-managed resources. Managed by platform administrators."
)

var ErrNoAdmin = errors.New("no active platform administrator to own the system workspace")

// FindSystem returns the built-in platform workspace.
func FindSystem(db *gorm.DB) (*models.Workspace, error) {
	var ws models.Workspace
	if err := db.Where("system = ?", true).First(&ws).Error; err != nil {
		return nil, err
	}
	return &ws, nil
}

// EnsureSystem creates the built-in platform workspace when it does not exist,
// owned by the lowest-id active administrator. Idempotent.
func EnsureSystem(db *gorm.DB) (*models.Workspace, error) {
	if ws, err := FindSystem(db); err == nil {
		return ws, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	var owner models.User
	if err := db.Where("role = ? AND active = ?", models.UserRoleAdmin, true).
		Order("id ASC").First(&owner).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNoAdmin
		}
		return nil, fmt.Errorf("resolve system workspace owner: %w", err)
	}

	slug, err := availableSlug(db, SystemSlug)
	if err != nil {
		return nil, err
	}

	ws := &models.Workspace{
		Name:        SystemName,
		Slug:        slug,
		Description: SystemDescription,
		OwnerID:     owner.ID,
		System:      true,
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(ws).Error; err != nil {
			return fmt.Errorf("create system workspace: %w", err)
		}
		member := &models.WorkspaceMember{
			WorkspaceID: ws.ID,
			UserID:      owner.ID,
			Role:        models.WorkspaceRoleOwner,
		}
		if err := tx.Create(member).Error; err != nil {
			return fmt.Errorf("create system workspace owner: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	logger.Info("platform system workspace ready", "workspace_id", ws.ID, "slug", ws.Slug, "owner_id", owner.ID)
	return ws, nil
}

// SyncMembers makes the system workspace's membership match the set of active
// platform administrators: every admin is a member, and nobody else is.
func SyncMembers(db *gorm.DB) error {
	ws, err := FindSystem(db)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	var adminIDs []uint
	if err := db.Model(&models.User{}).
		Where("role = ? AND active = ?", models.UserRoleAdmin, true).
		Pluck("id", &adminIDs).Error; err != nil {
		return fmt.Errorf("list admins: %w", err)
	}
	if len(adminIDs) == 0 {
		return nil
	}

	for _, id := range adminIDs {
		role := models.WorkspaceRoleAdmin
		if id == ws.OwnerID {
			role = models.WorkspaceRoleOwner
		}
		member := models.WorkspaceMember{WorkspaceID: ws.ID, UserID: id, Role: role}
		if err := db.Where("workspace_id = ? AND user_id = ?", ws.ID, id).
			FirstOrCreate(&member).Error; err != nil {
			return fmt.Errorf("add admin %d to system workspace: %w", id, err)
		}
	}

	return db.Where("workspace_id = ? AND user_id NOT IN ?", ws.ID, adminIDs).
		Delete(&models.WorkspaceMember{}).Error
}

// IsReservedSlug reports whether slug is claimable only by the system workspace.
func IsReservedSlug(slug string) bool { return slug == SystemSlug }

func availableSlug(db *gorm.DB, base string) (string, error) {
	slug := base
	for i := 2; i < 100; i++ {
		var count int64
		if err := db.Model(&models.Workspace{}).Where("slug = ?", slug).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return slug, nil
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
	return "", fmt.Errorf("could not find an available slug for %q", base)
}
