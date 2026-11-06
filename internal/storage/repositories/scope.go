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
	"github.com/jkaninda/logger"
	"gorm.io/gorm"
)

type ResourceScope struct {
	UserID      uint
	WorkspaceID *uint
}

// ApplyScope matches nothing without a workspace; UserID is for attribution only and never selects rows.
func ApplyScope(db *gorm.DB, scope ResourceScope) *gorm.DB {
	if scope.WorkspaceID == nil {
		logger.Warn("ApplyScope: request has no workspace; matching nothing", "user_id", scope.UserID)
		return db.Where("1 = 0")
	}
	return db.Where("workspace_id = ?", *scope.WorkspaceID)
}

func OwnsResource(scope ResourceScope, _ uint, resourceWorkspaceID *uint) bool {
	return OwnsWorkspaceResource(scope, resourceWorkspaceID)
}

// ApplyWorkspaceScope is an alias for ApplyScope, kept for one release so older call sites keep compiling.
func ApplyWorkspaceScope(db *gorm.DB, scope ResourceScope) *gorm.DB {
	return ApplyScope(db, scope)
}

func OwnsWorkspaceResource(scope ResourceScope, resourceWorkspaceID *uint) bool {
	return scope.WorkspaceID != nil && resourceWorkspaceID != nil && *scope.WorkspaceID == *resourceWorkspaceID
}
