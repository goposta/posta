// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import (
	"time"

	"github.com/lib/pq"
)

type SMTPCredential struct {
	ID           uint           `json:"id" gorm:"primaryKey"`
	WorkspaceID  uint           `json:"workspace_id" gorm:"index;not null"`
	UserID       uint           `json:"user_id" gorm:"index;not null"` // creator, for audit
	Name         string         `json:"name" gorm:"not null"`
	Username     string         `json:"username" gorm:"uniqueIndex;not null"`
	PasswordHash string         `json:"-" gorm:"not null"`
	AllowedIPs   pq.StringArray `json:"allowed_ips" gorm:"type:text[]"`
	Revoked      bool           `json:"revoked" gorm:"default:false"`
	CreatedAt    time.Time      `json:"created_at"`
	LastUsedAt   *time.Time     `json:"last_used_at"`

	User User `json:"-" gorm:"foreignKey:UserID"`
}

func (k *SMTPCredential) IsValid() bool {
	return !k.Revoked
}
