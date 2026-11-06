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

import (
	"time"

	"github.com/goposta/posta/internal/services/crypto"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

const (
	EncryptionNone     = "none"     // Plain text (port 25)
	EncryptionSTARTTLS = "starttls" // Upgrade via STARTTLS (port 587)
	EncryptionSSL      = "ssl"      // Implicit TLS (port 465)
)

const (
	SMTPStatusEnabled  = "enabled"  // Valid and available for sending
	SMTPStatusDisabled = "disabled" // Manually disabled by user
	SMTPStatusInvalid  = "invalid"  // Validation failed (bad credentials, connection error)
)

type SMTPServer struct {
	ID          uint  `json:"id" gorm:"primaryKey"`
	UserID      uint  `json:"user_id" gorm:"index;not null"`
	WorkspaceID *uint `json:"workspace_id,omitempty" gorm:"index"`
	// Name is an optional label to tell a workspace's servers apart; existing rows keep an empty name.
	Name string `json:"name"`
	// IsSystem finds the POSTA_SYSTEM_SMTP_* row on reboot, so an operator-added server is never overwritten.
	IsSystem        bool           `json:"is_system" gorm:"not null;default:false;index"`
	Host            string         `json:"host" gorm:"not null"`
	Port            int            `json:"port" gorm:"not null"`
	Username        string         `json:"username"`
	Password        string         `json:"-"`
	Encryption      string         `json:"encryption" gorm:"default:none;not null"`
	MaxRetries      int            `json:"max_retries" gorm:"default:0;not null"`
	AllowedEmails   pq.StringArray `json:"allowed_emails" gorm:"type:text[]"`
	Status          string         `json:"status" gorm:"default:enabled;not null"`
	ValidationError string         `json:"validation_error" gorm:"type:text"`
	ValidatedAt     *time.Time     `json:"validated_at"`
	CreatedAt       time.Time      `json:"created_at"`

	User User `json:"-" gorm:"foreignKey:UserID"`
}

func (s *SMTPServer) IsEnabled() bool {
	return s.Status == SMTPStatusEnabled
}

// BeforeSave encrypts the password with AES-256-GCM if POSTA_ENCRYPTION_KEY is set, otherwise only base64.
func (s *SMTPServer) BeforeSave(tx *gorm.DB) error {
	if s.Password == "" || crypto.IsEncrypted(s.Password) {
		return nil
	}
	encrypted, err := crypto.Encrypt(s.Password)
	if err != nil {
		return err
	}
	s.Password = encrypted
	return nil
}

func (s *SMTPServer) AfterFind(tx *gorm.DB) error {
	if s.Password == "" {
		return nil
	}
	plaintext, err := crypto.Decrypt(s.Password)
	if err != nil {
		return nil // degrade gracefully
	}
	s.Password = plaintext
	return nil
}
