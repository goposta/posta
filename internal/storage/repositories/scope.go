// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package repositories

import (
	"github.com/jkaninda/logger"
	"gorm.io/gorm"
)

type ResourceScope struct {
	UserID      uint
	WorkspaceID *uint
}

// ApplyScope narrows a query to the caller's workspace. Every resource is
// workspace-scoped; a scope without one matches nothing rather than falling back
// to the pre-workspace "user_id AND workspace_id IS NULL" rows, which no longer
// exist. UserID remains on the scope for attribution, and never selects rows.
func ApplyScope(db *gorm.DB, scope ResourceScope) *gorm.DB {
	if scope.WorkspaceID == nil {
		logger.Warn("ApplyScope: request has no workspace; matching nothing", "user_id", scope.UserID)
		return db.Where("1 = 0")
	}
	return db.Where("workspace_id = ?", *scope.WorkspaceID)
}

// OwnsResource checks whether the given resource belongs to the current scope.
func OwnsResource(scope ResourceScope, _ uint, resourceWorkspaceID *uint) bool {
	return OwnsWorkspaceResource(scope, resourceWorkspaceID)
}

// ApplyWorkspaceScope is an alias for ApplyScope, kept for one release so the
// call sites added while the two behaved differently keep compiling.
func ApplyWorkspaceScope(db *gorm.DB, scope ResourceScope) *gorm.DB {
	return ApplyScope(db, scope)
}

func OwnsWorkspaceResource(scope ResourceScope, resourceWorkspaceID *uint) bool {
	return scope.WorkspaceID != nil && resourceWorkspaceID != nil && *scope.WorkspaceID == *resourceWorkspaceID
}
