// SPDX-FileCopyrightText: 2026 Jonas Kaninda
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
	Name            string `json:"name" gorm:"not null"`
	Slug            string `json:"slug" gorm:"uniqueIndex;not null"`
	Description     string `json:"description"`
	OwnerID         uint   `json:"owner_id" gorm:"index;not null"`
	PlanID          *uint  `json:"plan_id" gorm:"index"`
	DefaultLanguage string `json:"default_language" gorm:"size:10;default:'en'"`
	// System marks the single built-in platform workspace. It owns
	// platform-managed resources, is created on first boot, cannot be renamed or
	// deleted, and admits only platform admins.
	System    bool      `json:"system" gorm:"not null;default:false"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

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

// IsSystem reports whether this is the built-in platform workspace.
func (w *Workspace) IsSystem() bool { return w.System }

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
