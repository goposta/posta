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

const (
	// ServerSecurityModePermissive allows any user whose sender domain matches
	// allowed_domains to use this server, regardless of domain ownership.
	ServerSecurityModePermissive = "permissive"

	// ServerSecurityModeStrict requires the user to have verified ownership of
	// the sender domain (via TXT record) before the server is available to them.
	ServerSecurityModeStrict = "strict"
)

// Server represents a shared SMTP server managed by platform administrators.
// Unlike per-user SMTPServer records, a Server is available across multiple
// user accounts based on the sender domain.
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

// IsEnabled returns true when the server can be used for delivery.
func (s *Server) IsEnabled() bool {
	return s.Status == SMTPStatusEnabled
}

// BeforeSave encrypts the password before writing to the database.
// Uses AES-256-GCM if POSTA_ENCRYPTION_KEY is set, otherwise base64.
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

// AfterFind decrypts the password after reading from the database.
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

// ToSMTPServer converts this shared Server into the SMTPServer format used
// by the SMTP sender, so existing send logic can be reused without modification.
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
