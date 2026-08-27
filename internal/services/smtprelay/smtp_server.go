// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package smtprelay

import (
	"fmt"
	"time"

	"github.com/emersion/go-smtp"
)

// SMTPConfig holds configuration for the built-in SMTP Relay listener.
type SMTPConfig struct {
	Host           string
	Port           int
	Hostname       string
	MaxMessageSize int64
}

// AllowInsecureAuth is required here: it's what permits AUTH PLAIN without TLS.
func NewSMTPServer(backend *Backend, cfg SMTPConfig) (*smtp.Server, error) {
	srv := smtp.NewServer(backend)
	srv.Addr = fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	srv.Domain = cfg.Hostname
	srv.ReadTimeout = 30 * time.Second
	srv.WriteTimeout = 30 * time.Second
	srv.MaxMessageBytes = cfg.MaxMessageSize
	srv.MaxRecipients = 50
	srv.AllowInsecureAuth = true
	srv.EnableSMTPUTF8 = true
	return srv, nil
}
