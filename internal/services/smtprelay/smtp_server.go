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

package smtprelay

import (
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/emersion/go-smtp"
)

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
