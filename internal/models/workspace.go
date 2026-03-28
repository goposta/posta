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

package models

import "time"

type WorkspaceRole string

const (
	WorkspaceRoleOwner  WorkspaceRole = "owner"
	WorkspaceRoleAdmin  WorkspaceRole = "admin"
	WorkspaceRoleEditor WorkspaceRole = "editor"
	WorkspaceRoleViewer WorkspaceRole = "viewer"
)

type InvitationStatus string

const (
	InvitationStatusPending  InvitationStatus = "pending"
	InvitationStatusAccepted InvitationStatus = "accepted"
	InvitationStatusDeclined InvitationStatus = "declined"
)

type Workspace struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	Name            string    `json:"name" gorm:"not null"`
	Slug            string    `json:"slug" gorm:"uniqueIndex;not null"`
	Description     string    `json:"description"`
	OwnerID         uint      `json:"owner_id" gorm:"index;not null"`
	PlanID          *uint     `json:"plan_id" gorm:"index"`
	DefaultLanguage string    `json:"default_language" gorm:"size:10;default:'en'"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`

	Owner   User              `json:"-" gorm:"foreignKey:OwnerID"`
	Plan    Plan              `json:"-" gorm:"foreignKey:PlanID"`
	Members []WorkspaceMember `json:"-" gorm:"foreignKey:WorkspaceID"`
}

type WorkspaceMember struct {
	ID          uint          `json:"id" gorm:"primaryKey"`
	WorkspaceID uint          `json:"workspace_id" gorm:"uniqueIndex:idx_workspace_user;not null"`
	UserID      uint          `json:"user_id" gorm:"uniqueIndex:idx_workspace_user;not null"`
	Role        WorkspaceRole `json:"role" gorm:"not null;default:viewer"`
	CreatedAt   time.Time     `json:"created_at"`

	Workspace Workspace `json:"-" gorm:"foreignKey:WorkspaceID"`
	User      User      `json:"-" gorm:"foreignKey:UserID"`
}

type WorkspaceInvitation struct {
	ID          uint             `json:"id" gorm:"primaryKey"`
	WorkspaceID uint             `json:"workspace_id" gorm:"index;not null"`
	Email       string           `json:"email" gorm:"not null"`
	Role        WorkspaceRole    `json:"role" gorm:"not null;default:viewer"`
	Token       string           `json:"-" gorm:"uniqueIndex;not null"`
	Status      InvitationStatus `json:"status" gorm:"not null;default:pending"`
	InvitedBy   uint             `json:"invited_by" gorm:"not null"`
	ExpiresAt   time.Time        `json:"expires_at" gorm:"not null"`
	CreatedAt   time.Time        `json:"created_at"`

	Workspace Workspace `json:"-" gorm:"foreignKey:WorkspaceID"`
	Inviter   User      `json:"-" gorm:"foreignKey:InvitedBy"`
}

// CanManageMembers returns true if the role can invite/remove members.
func (r WorkspaceRole) CanManageMembers() bool {
	return r == WorkspaceRoleOwner || r == WorkspaceRoleAdmin
}

// CanEdit returns true if the role can create/modify resources.
func (r WorkspaceRole) CanEdit() bool {
	return r == WorkspaceRoleOwner || r == WorkspaceRoleAdmin || r == WorkspaceRoleEditor
}

// CanView returns true if the role has any access.
func (r WorkspaceRole) CanView() bool {
	return r == WorkspaceRoleOwner || r == WorkspaceRoleAdmin || r == WorkspaceRoleEditor || r == WorkspaceRoleViewer
}

// IsOwner returns true if the role is owner.
func (r WorkspaceRole) IsOwner() bool {
	return r == WorkspaceRoleOwner
}
