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

	// DefaultWorkspaceID is where an unscoped request lands; repaired when the user has left that workspace.
	DefaultWorkspaceID *uint      `json:"default_workspace_id" gorm:"index"`
	MigratedAt         *time.Time `json:"-"`
	MigrationError     string     `json:"-" gorm:"type:text"`

	Plan Plan `json:"-" gorm:"foreignKey:PlanID"`
}

func (u *User) IsAdmin() bool {
	return u.Role == UserRoleAdmin
}
