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

package workspace

import (
	"errors"
	"fmt"

	"github.com/goposta/posta/internal/config"
	"github.com/goposta/posta/internal/models"
	"github.com/jkaninda/logger"
	"gorm.io/gorm"
)

const (
	SystemName        = "system"
	SystemDisplayName = "Posta System"
	SystemDescription = "Built-in workspace for platform-managed resources. Managed by platform administrators."
)

var ErrNoAdmin = errors.New("no active platform administrator to own the system workspace")

func FindSystem(db *gorm.DB) (*models.Workspace, error) {
	var ws models.Workspace
	if err := db.Where("system = ?", true).First(&ws).Error; err != nil {
		return nil, err
	}
	return &ws, nil
}

// EnsureSystem is idempotent: on later boots it only re-syncs the system SMTP connection settings.
func EnsureSystem(db *gorm.DB, smtp config.SystemSMTPConfig) (*models.Workspace, error) {
	if ws, err := FindSystem(db); err == nil {
		if serr := db.Transaction(func(tx *gorm.DB) error {
			return provisionSystemSMTP(tx, ws, ws.OwnerID, smtp)
		}); serr != nil {
			return nil, serr
		}
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

	name, err := availableName(db, SystemName)
	if err != nil {
		return nil, err
	}

	ws := &models.Workspace{
		DisplayName: SystemDisplayName,
		Name:        name,
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

		settings := &models.WorkspaceSetting{
			WorkspaceID:        ws.ID,
			Timezone:           "UTC",
			WebhookRetryCount:  3,
			APIKeyExpiryDays:   90,
			BounceAutoSuppress: true,
		}
		// The platform From is the sending default so a dashboard test send uses the same identity.
		if name, addr, perr := ParseSender(smtp.From); perr == nil {
			settings.DefaultSenderName = name
			settings.DefaultSenderEmail = addr
		}
		if err := tx.Create(settings).Error; err != nil {
			return fmt.Errorf("create system workspace settings: %w", err)
		}

		return provisionSystemSMTP(tx, ws, owner.ID, smtp)
	})
	if err != nil {
		return nil, err
	}

	logger.Info("platform system workspace ready", "workspace_id", ws.ID, "name", ws.Name, "owner_id", owner.ID)
	return ws, nil
}

// SyncMembers makes every active platform admin a member of the system workspace, and nobody else.
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

// IsReservedName reports whether name is claimable only by the system workspace.
func IsReservedName(name string) bool { return name == SystemName }

func availableName(db *gorm.DB, base string) (string, error) {
	name := base
	for i := 2; i < 100; i++ {
		var count int64
		if err := db.Model(&models.Workspace{}).Where("name = ?", name).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return name, nil
		}
		name = fmt.Sprintf("%s-%d", base, i)
	}
	return "", fmt.Errorf("could not find an available name for %q", base)
}
