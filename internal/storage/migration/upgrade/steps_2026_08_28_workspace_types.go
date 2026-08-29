// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package upgrade

import (
	"errors"
	"fmt"

	"github.com/goposta/posta/internal/config"
	"github.com/goposta/posta/internal/services/workspace"
	"github.com/jkaninda/logger"
	"gorm.io/gorm"
)

// applySystemWorkspace creates the built-in platform workspace. A install with
// no administrator yet (fresh database, admin seeded later) is not an error:
// the server calls EnsureSystem again once seeding completes.
func applySystemWorkspace(tx *gorm.DB) error {
	if err := tx.Exec(`ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS system boolean NOT NULL DEFAULT false`).Error; err != nil {
		return fmt.Errorf("add workspaces.system: %w", err)
	}
	if err := tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS one_system_workspace ON workspaces ((true)) WHERE system`).Error; err != nil {
		return fmt.Errorf("create one_system_workspace: %w", err)
	}

	// No SMTP configuration here on purpose: this step runs before crypto.Init,
	// so a password encrypted now would use an uninitialised key. The workspace
	// is all this step needs; the server is provisioned at boot, after seeding.
	if _, err := workspace.EnsureSystem(tx, config.SystemSMTPConfig{}); err != nil {
		if errors.Is(err, workspace.ErrNoAdmin) {
			logger.Info("system workspace deferred: no administrator yet")
			return nil
		}
		return err
	}
	return nil
}

// applyPersonalToNormal turns every personal workspace into an ordinary one.
// Nothing moves: ids, slugs, members, plans, and resources are untouched.
func applyPersonalToNormal(tx *gorm.DB) error {
	if !hasColumn(tx, "workspaces", "is_personal") {
		return nil
	}
	res := tx.Exec(`UPDATE workspaces SET is_personal = false WHERE is_personal = true`)
	if res.Error != nil {
		return fmt.Errorf("flatten personal workspaces: %w", res.Error)
	}
	if res.RowsAffected > 0 {
		logger.Info("workspace types: personal workspaces converted to normal", "count", res.RowsAffected)
	}
	return tx.Exec(`DROP INDEX IF EXISTS one_personal_per_user`).Error
}

// applyDefaultWorkspace renames personal_workspace_id to default_workspace_id so
// every user keeps landing exactly where they landed before, then repairs any
// value that points at a workspace the user is not a member of.
func applyDefaultWorkspace(tx *gorm.DB) error {
	switch {
	case hasColumn(tx, "users", "default_workspace_id"):
		// Already renamed, or created fresh by AutoMigrate.
		if hasColumn(tx, "users", "personal_workspace_id") {
			if err := tx.Exec(`
				UPDATE users SET default_workspace_id = personal_workspace_id
				WHERE default_workspace_id IS NULL AND personal_workspace_id IS NOT NULL`).Error; err != nil {
				return fmt.Errorf("copy personal_workspace_id: %w", err)
			}
		}
	case hasColumn(tx, "users", "personal_workspace_id"):
		if err := tx.Exec(`ALTER TABLE users RENAME COLUMN personal_workspace_id TO default_workspace_id`).Error; err != nil {
			return fmt.Errorf("rename personal_workspace_id: %w", err)
		}
	default:
		return nil
	}

	res := tx.Exec(`
		UPDATE users u SET default_workspace_id = (
			SELECT m.workspace_id FROM workspace_members m
			JOIN workspaces w ON w.id = m.workspace_id AND w.system = false
			WHERE m.user_id = u.id
			ORDER BY m.created_at ASC, m.workspace_id ASC
			LIMIT 1
		)
		WHERE u.default_workspace_id IS NOT NULL
		  AND NOT EXISTS (
			SELECT 1 FROM workspace_members m
			WHERE m.user_id = u.id AND m.workspace_id = u.default_workspace_id
		  )`)
	if res.Error != nil {
		return fmt.Errorf("repair dangling default workspaces: %w", res.Error)
	}
	if res.RowsAffected > 0 {
		logger.Info("workspace types: dangling default workspaces repaired", "count", res.RowsAffected)
	}
	return nil
}

func hasColumn(tx *gorm.DB, table, column string) bool {
	var count int64
	tx.Raw(`SELECT count(*) FROM information_schema.columns WHERE table_name = ? AND column_name = ?`,
		table, column).Scan(&count)
	return count > 0
}
