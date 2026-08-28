// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

type UserRole string

const (
	UserRoleAdmin UserRole = "admin"
	UserRoleUser  UserRole = "user"
)

type User struct {
	ID                    uint       `json:"id" gorm:"primaryKey"`
	Name                  string     `json:"name" gorm:"not null;default:''"`
	Email                 string     `json:"email" gorm:"uniqueIndex;not null"`
	PasswordHash          string     `json:"-" gorm:"not null"`
	Role                  UserRole   `json:"role" gorm:"default:user;not null"`
	TwoFactorSecret       string     `json:"-" gorm:"type:text"`
	TwoFactorEnabled      bool       `json:"two_factor_enabled" gorm:"default:false"`
	Active                bool       `json:"active" gorm:"default:true;not null"`
	RequireVerifiedDomain bool       `json:"require_verified_domain" gorm:"default:false"`
	AuthMethod            string     `json:"auth_method" gorm:"default:'password';not null"`
	AvatarURL             string     `json:"avatar_url"`
	PlanID                *uint      `json:"plan_id" gorm:"index"`
	ScheduledDeletionAt   *time.Time `json:"scheduled_deletion_at"`
	EmailVerifiedAt       *time.Time `json:"email_verified_at"`
	CreatedAt             time.Time  `json:"created_at"`
	LastLoginAt           *time.Time `json:"last_login_at"`

	// DefaultWorkspaceID is where a request that names no workspace lands. It is
	// user-settable and repaired when it points at a workspace the user has left.
	DefaultWorkspaceID *uint      `json:"default_workspace_id" gorm:"index"`
	MigratedAt         *time.Time `json:"-"`
	MigrationError     string     `json:"-" gorm:"type:text"`

	Plan Plan `json:"-" gorm:"foreignKey:PlanID"`
}

func (u *User) IsAdmin() bool {
	return u.Role == UserRoleAdmin
}
