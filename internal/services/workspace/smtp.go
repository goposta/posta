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

package workspace

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/goposta/posta/internal/config"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/crypto"
	"github.com/jkaninda/logger"
	"gorm.io/gorm"
)

// SystemSMTPServerName is only a label: the row is found by its IsSystem marker, so it may be renamed.
const SystemSMTPServerName = "System SMTP"

// connectionColumns are re-synced from config on every boot; the rest of the row belongs to the operator.
var connectionColumns = []string{"host", "port", "username", "encryption", "password"}

// provisionSystemSMTP treats unusable config as no server: notifications then send straight from config.
func provisionSystemSMTP(tx *gorm.DB, ws *models.Workspace, ownerID uint, cfg config.SystemSMTPConfig) error {
	existing, err := findSystemSMTP(tx, ws.ID)
	if err != nil {
		return err
	}

	if !cfg.IsConfigured() {
		if existing != nil {
			logger.Warn("system workspace: POSTA_SYSTEM_SMTP_* is no longer configured; "+
				"the provisioned server keeps its stored settings until it is removed",
				"workspace_id", ws.ID, "smtp_server_id", existing.ID)
		}
		return nil
	}

	if existing == nil {
		server := &models.SMTPServer{
			UserID:      ownerID,
			WorkspaceID: &ws.ID,
			Name:        SystemSMTPServerName,
			IsSystem:    true,
			Host:        cfg.Host,
			Port:        cfg.Port,
			Username:    cfg.Username,
			Password:    cfg.Password,
			Encryption:  NormalizeEncryption(cfg.Encryption),
			Status:      models.SMTPStatusEnabled,
		}
		if err := tx.Create(server).Error; err != nil {
			return fmt.Errorf("create system SMTP server: %w", err)
		}
		logger.Info("system workspace: provisioned SMTP server",
			"workspace_id", ws.ID, "smtp_server_id", server.ID,
			"host", server.Host, "port", server.Port, "encryption", server.Encryption)
		return nil
	}

	updates := connectionDrift(cfg, existing)
	if len(updates) == 0 {
		return nil
	}
	if err := tx.Model(&models.SMTPServer{}).Where("id = ?", existing.ID).Updates(updates).Error; err != nil {
		return fmt.Errorf("sync system SMTP server: %w", err)
	}
	logger.Info("system workspace: synced SMTP connection settings from configuration",
		"smtp_server_id", existing.ID, "fields", strings.Join(changedColumns(updates), ","))
	return nil
}

// findSystemSMTP uses the IsSystem marker so an operator-added server is never overwritten from env.
func findSystemSMTP(tx *gorm.DB, workspaceID uint) (*models.SMTPServer, error) {
	var server models.SMTPServer
	err := tx.Where("workspace_id = ? AND is_system = ?", workspaceID, true).
		Order("id").First(&server).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("look up system SMTP server: %w", err)
	}
	return &server, nil
}

// connectionDrift re-encrypts the password itself: a map-based Updates call bypasses BeforeSave.
func connectionDrift(cfg config.SystemSMTPConfig, existing *models.SMTPServer) map[string]any {
	updates := map[string]any{}
	if existing.Host != cfg.Host {
		updates["host"] = cfg.Host
	}
	if existing.Port != cfg.Port {
		updates["port"] = cfg.Port
	}
	if existing.Username != cfg.Username {
		updates["username"] = cfg.Username
	}
	if enc := NormalizeEncryption(cfg.Encryption); existing.Encryption != enc {
		updates["encryption"] = enc
	}
	if cfg.Password != "" && existing.Password != cfg.Password {
		encrypted, err := crypto.Encrypt(cfg.Password)
		if err != nil {
			logger.Error("system workspace: could not encrypt the SMTP password; "+
				"leaving the stored one in place", "error", err)
		} else {
			updates["password"] = encrypted
		}
	}
	return updates
}

// changedColumns reports the password's presence, never its value.
func changedColumns(updates map[string]any) []string {
	out := make([]string, 0, len(updates))
	for _, col := range connectionColumns {
		if _, ok := updates[col]; ok {
			out = append(out, col)
		}
	}
	return out
}

// NormalizeEncryption maps "tls" to implicit TLS and defaults to STARTTLS, so a typo can't fail at delivery.
func NormalizeEncryption(enc string) string {
	switch strings.ToLower(strings.TrimSpace(enc)) {
	case models.EncryptionNone:
		return models.EncryptionNone
	case models.EncryptionSSL, "tls":
		return models.EncryptionSSL
	default:
		return models.EncryptionSTARTTLS
	}
}

// ParseSender accepts both a bare address and the RFC 5322 display-name form.
func ParseSender(from string) (name, addr string, err error) {
	from = strings.TrimSpace(from)
	if from == "" {
		return "", "", errors.New("sender address is empty")
	}
	if parsed, perr := mail.ParseAddress(from); perr == nil {
		return parsed.Name, parsed.Address, nil
	}
	if !strings.Contains(from, "@") {
		return "", "", fmt.Errorf("sender address %q has no domain", from)
	}
	return "", from, nil
}
