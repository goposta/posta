// SPDX-FileCopyrightText: 2026 Jonas Kaninda
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

// SystemSMTPServerName labels the server provisioned from POSTA_SYSTEM_SMTP_*.
// Only the label: the row is located by its IsSystem marker, so an operator may
// rename it freely.
const SystemSMTPServerName = "System SMTP"

// connectionColumns are owned by configuration and re-synced on every boot, so
// rotating a credential in the environment takes effect on restart. Everything
// else on the row — the label, the status, retry and recipient limits — belongs
// to the operator and is written once at creation.
var connectionColumns = []string{"host", "port", "username", "encryption", "password"}

// provisionSystemSMTP creates the system workspace's SMTP server from
// configuration, or re-syncs the connection settings of the one already there.
//
// An unconfigured or unusable POSTA_SYSTEM_SMTP_* is not an error: the platform
// boots without a server and the notification service falls back to sending
// straight from configuration.
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

// findSystemSMTP locates the provisioned server by its marker rather than by
// name or position, so a server the operator adds to the same workspace is
// never adopted and overwritten from the environment.
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

// connectionDrift returns the config-owned columns whose stored value differs
// from configuration. An empty map means the row is already in sync and no
// write happens.
//
// The password compares in plaintext because models.SMTPServer decrypts it on
// read, and is re-encrypted explicitly here: a map-based Updates call bypasses
// BeforeSave, so without this the secret would be stored in the clear.
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

// changedColumns lists updated column names in a stable order for logging. The
// password's presence is reported; its value never is.
func changedColumns(updates map[string]any) []string {
	out := make([]string, 0, len(updates))
	for _, col := range connectionColumns {
		if _, ok := updates[col]; ok {
			out = append(out, col)
		}
	}
	return out
}

// NormalizeEncryption maps a configured value onto a mode the sender
// understands, treating "tls" as a synonym for implicit TLS and defaulting to
// STARTTLS as the configuration default does. Without it a typo produces a row
// the sender rejects at delivery time rather than at boot.
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

// ParseSender splits POSTA_SYSTEM_SMTP_FROM into a display name and an address,
// accepting both a bare address and RFC 5322 display-name form.
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
