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

import (
	"time"

	"github.com/goposta/posta/internal/services/crypto"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

// Encryption modes for SMTP connections.
const (
	EncryptionNone     = "none"     // Plain text (port 25)
	EncryptionSTARTTLS = "starttls" // Upgrade via STARTTLS (port 587)
	EncryptionSSL      = "ssl"      // Implicit TLS (port 465)
)

// SMTP server status values.
const (
	SMTPStatusEnabled  = "enabled"  // Valid and available for sending
	SMTPStatusDisabled = "disabled" // Manually disabled by user
	SMTPStatusInvalid  = "invalid"  // Validation failed (bad credentials, connection error)
)

type SMTPServer struct {
	ID              uint           `json:"id" gorm:"primaryKey"`
	UserID          uint           `json:"user_id" gorm:"index;not null"`
	WorkspaceID     *uint          `json:"workspace_id,omitempty" gorm:"index"`
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

// IsEnabled returns true when the server can be used for delivery.
func (s *SMTPServer) IsEnabled() bool {
	return s.Status == SMTPStatusEnabled
}

// BeforeSave encrypts the password before writing to the database.
// Uses AES-256-GCM if POSTA_ENCRYPTION_KEY is set, otherwise base64.
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

// AfterFind decrypts the password after reading from the database.
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
