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
	// ServerSecurityModePermissive allows any user whose sender domain matches allowed_domains, ownership aside.
	ServerSecurityModePermissive = "permissive"

	// ServerSecurityModeStrict requires TXT-verified ownership of the sender domain before the server is offered.
	ServerSecurityModeStrict = "strict"
)

// Server is a platform-managed SMTP server shared across accounts by sender domain, unlike per-user SMTPServer.
type Server struct {
	ID              uint           `json:"id" gorm:"primaryKey"`
	Name            string         `json:"name" gorm:"not null"`
	Host            string         `json:"host" gorm:"not null"`
	Port            int            `json:"port" gorm:"not null"`
	Username        string         `json:"username"`
	Password        string         `json:"-"`
	Encryption      string         `json:"encryption" gorm:"default:none;not null"`
	MaxRetries      int            `json:"max_retries" gorm:"default:0;not null"`
	Status          string         `json:"status" gorm:"default:enabled;not null"`
	AllowedDomains  pq.StringArray `json:"allowed_domains" gorm:"type:text[]"`
	SecurityMode    string         `json:"security_mode" gorm:"default:permissive;not null"`
	SentCount       int64          `json:"sent_count" gorm:"default:0;not null"`
	FailedCount     int64          `json:"failed_count" gorm:"default:0;not null"`
	ValidationError string         `json:"validation_error" gorm:"type:text"`
	ValidatedAt     *time.Time     `json:"validated_at"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

func (s *Server) IsEnabled() bool {
	return s.Status == SMTPStatusEnabled
}

// BeforeSave encrypts the password with AES-256-GCM if POSTA_ENCRYPTION_KEY is set, otherwise only base64.
func (s *Server) BeforeSave(tx *gorm.DB) error {
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

func (s *Server) AfterFind(tx *gorm.DB) error {
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

// ToSMTPServer adapts this Server to the SMTPServer shape the SMTP sender already uses.
func (s *Server) ToSMTPServer() *SMTPServer {
	return &SMTPServer{
		ID:         s.ID,
		Host:       s.Host,
		Port:       s.Port,
		Username:   s.Username,
		Password:   s.Password,
		Encryption: s.Encryption,
		MaxRetries: s.MaxRetries,
	}
}
