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
	CreatedAt             time.Time  `json:"created_at"`
	LastLoginAt           *time.Time `json:"last_login_at"`
}

func (u *User) IsAdmin() bool {
	return u.Role == UserRoleAdmin
}
