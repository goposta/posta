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
	"github.com/jkaninda/logger"
	"gorm.io/gorm"
)

type ResourceScope struct {
	UserID      uint
	WorkspaceID *uint
}

var workspaceOnlyMode bool

// SetWorkspaceOnlyMode enables or disables the R6 personal-mode lock.
func SetWorkspaceOnlyMode(enabled bool) { workspaceOnlyMode = enabled }

func ApplyScope(db *gorm.DB, scope ResourceScope) *gorm.DB {
	if scope.WorkspaceID != nil {
		return db.Where("workspace_id = ?", *scope.WorkspaceID)
	}
	if workspaceOnlyMode {
		logger.Warn("ApplyScope: personal-mode scope rejected (workspace-only mode)", "user_id", scope.UserID)
		return db.Where("1 = 0")
	}
	return db.Where("user_id = ? AND workspace_id IS NULL", scope.UserID)
}

// OwnsResource checks whether the given resource belongs to the current scope.
func OwnsResource(scope ResourceScope, resourceUserID uint, resourceWorkspaceID *uint) bool {
	if scope.WorkspaceID != nil {
		return resourceWorkspaceID != nil && *resourceWorkspaceID == *scope.WorkspaceID
	}
	return resourceUserID == scope.UserID && resourceWorkspaceID == nil
}
