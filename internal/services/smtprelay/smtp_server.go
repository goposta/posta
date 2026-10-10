// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package smtprelay

import (
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/emersion/go-smtp"
)

// SMTPConfig holds configuration for the built-in SMTP Relay listener.
type SMTPConfig struct {
	Host           string
	Port           int
	Hostname       string
	MaxMessageSize int64
	TLSMode        string // "none" or "starttls"
	TLSCertFile    string
	TLSKeyFile     string
}

func NewSMTPServer(backend *Backend, cfg SMTPConfig) (*smtp.Server, error) {
	srv := smtp.NewServer(backend)
	srv.Addr = fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	srv.Domain = cfg.Hostname
	srv.ReadTimeout = 30 * time.Second
	srv.WriteTimeout = 30 * time.Second
	srv.MaxMessageBytes = cfg.MaxMessageSize
	srv.MaxRecipients = 50
	srv.EnableSMTPUTF8 = true
	switch strings.ToLower(strings.TrimSpace(cfg.TLSMode)) {
	case "", "none":
		// Preserve the existing private-network plaintext relay by default.
		srv.AllowInsecureAuth = true
	case "starttls":
		cert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
		if err != nil {
			return nil, fmt.Errorf("load SMTP relay TLS cert: %w", err)
		}
		srv.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}
	default:
		return nil, fmt.Errorf("unsupported SMTP relay TLS mode %q (use none or starttls)", cfg.TLSMode)
	}
	return srv, nil
}
