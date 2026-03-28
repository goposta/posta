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

import "gorm.io/gorm"

// ResourceScope defines the ownership context for a request.
// If WorkspaceID is nil, the request is in personal mode.
// If WorkspaceID is set, the request targets that workspace.
type ResourceScope struct {
	UserID      uint
	WorkspaceID *uint
}

// ApplyScope adds the appropriate WHERE clause to a GORM query.
// Personal: user_id = ? AND workspace_id IS NULL
// Workspace: workspace_id = ?
func ApplyScope(db *gorm.DB, scope ResourceScope) *gorm.DB {
	if scope.WorkspaceID != nil {
		return db.Where("workspace_id = ?", *scope.WorkspaceID)
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
