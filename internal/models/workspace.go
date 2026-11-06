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
	ID              uint   `json:"id" gorm:"primaryKey"`
	Name            string `json:"name" gorm:"uniqueIndex;not null"`
	DisplayName     string `json:"display_name" gorm:"not null;default:''"`
	Description     string `json:"description"`
	OwnerID         uint   `json:"owner_id" gorm:"index;not null"`
	PlanID          *uint  `json:"plan_id" gorm:"index"`
	DefaultLanguage string `json:"default_language" gorm:"size:10;default:'en'"`
	// System marks the one built-in platform workspace: created on first boot, undeletable, platform admins only.
	System bool `json:"system" gorm:"not null;default:false"`
	// HostedFormsDisabled takes hosted /f/{key} pages offline (e.g. for phishing); embedded forms keep working.
	HostedFormsDisabled bool      `json:"hosted_forms_disabled" gorm:"not null;default:false"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`

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

func (w *Workspace) IsSystem() bool { return w.System }

func (r WorkspaceRole) CanManageMembers() bool {
	return r == WorkspaceRoleOwner || r == WorkspaceRoleAdmin
}

func (r WorkspaceRole) CanEdit() bool {
	return r == WorkspaceRoleOwner || r == WorkspaceRoleAdmin || r == WorkspaceRoleEditor
}

func (r WorkspaceRole) CanView() bool {
	return r == WorkspaceRoleOwner || r == WorkspaceRoleAdmin || r == WorkspaceRoleEditor || r == WorkspaceRoleViewer
}

func (r WorkspaceRole) IsOwner() bool {
	return r == WorkspaceRoleOwner
}
