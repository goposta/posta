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

package inbound

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/mail"
	"strings"
	"time"

	"github.com/emersion/go-smtp"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/logger"
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

type Backend struct {
	svc            *Service
	domainRepo     *repositories.DomainRepository
	maxMessageSize int64
	limiter        *IPRateLimiter
}

func NewBackend(svc *Service, domainRepo *repositories.DomainRepository, maxMessageSize int64) *Backend {
	return &Backend{svc: svc, domainRepo: domainRepo, maxMessageSize: maxMessageSize}
}

func (b *Backend) SetRateLimiter(l *IPRateLimiter) { b.limiter = l }

func (b *Backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	remote := c.Conn().RemoteAddr().String()
	if b.limiter != nil {
		ip := remote
		if host, _, err := net.SplitHostPort(remote); err == nil {
			ip = host
		}
		if !b.limiter.Allow(ip) {
			logger.Warn("smtp: rate-limited remote", "ip", ip)
			return nil, &smtp.SMTPError{Code: 421, EnhancedCode: smtp.EnhancedCode{4, 7, 0}, Message: "rate limit exceeded"}
		}
	}
	return &session{
		backend: b,
		remote:  remote,
	}, nil
}

type session struct {
	backend *Backend
	from    string
	to      []string
	remote  string
}

func (s *session) Reset() {
	s.from = ""
	s.to = nil
}

func (s *session) Logout() error { return nil }

func (s *session) Mail(from string, _ *smtp.MailOptions) error {
	if from != "" {
		if addr, err := mail.ParseAddress(from); err == nil {
			s.from = addr.Address
		} else {
			s.from = from
		}
	}
	return nil
}

// Rcpt rejects unverified domains before DATA to save bandwidth.
func (s *session) Rcpt(to string, _ *smtp.RcptOptions) error {
	addr := to
	if parsed, err := mail.ParseAddress(to); err == nil {
		addr = parsed.Address
	}
	domain := extractDomain(addr)
	if domain == "" {
		return &smtp.SMTPError{Code: 550, EnhancedCode: smtp.EnhancedCode{5, 1, 3}, Message: "malformed recipient"}
	}
	if _, err := s.backend.domainRepo.FindVerifiedByName(domain); err != nil {
		return &smtp.SMTPError{Code: 550, EnhancedCode: smtp.EnhancedCode{5, 1, 1}, Message: "relay not permitted"}
	}
	s.to = append(s.to, addr)
	return nil
}

// Parsing is deferred to the worker, so once IngestRaw succeeds the message is durable and the MTA gets 250.
func (s *session) Data(r io.Reader) error {
	limit := s.backend.maxMessageSize
	if limit <= 0 {
		limit = 26214400
	}
	// +1 to detect overflow.
	lr := &io.LimitedReader{R: r, N: limit + 1}
	raw, err := io.ReadAll(lr)
	if err != nil {
		return &smtp.SMTPError{Code: 451, EnhancedCode: smtp.EnhancedCode{4, 3, 0}, Message: "read error"}
	}
	if int64(len(raw)) > limit {
		return &smtp.SMTPError{Code: 552, EnhancedCode: smtp.EnhancedCode{5, 3, 4}, Message: "message size exceeds limit"}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	recipients := append([]string(nil), s.to...)
	_, ierr := s.backend.svc.IngestRaw(ctx, raw, s.from, recipients, models.InboundSourceSMTP)
	switch {
	case ierr == nil:
		return nil
	case errors.Is(ierr, ErrUnverifiedDomain):
		return &smtp.SMTPError{Code: 550, EnhancedCode: smtp.EnhancedCode{5, 1, 1}, Message: "relay not permitted"}
	case errors.Is(ierr, ErrSizeExceeded):
		return &smtp.SMTPError{Code: 552, EnhancedCode: smtp.EnhancedCode{5, 3, 4}, Message: "message too large"}
	case errors.Is(ierr, ErrSenderSuppressed):
		return &smtp.SMTPError{Code: 550, EnhancedCode: smtp.EnhancedCode{5, 7, 1}, Message: "sender suppressed"}
	default:
		logger.Error("inbound ingest failed", "remote", s.remote, "error", ierr)
		return &smtp.SMTPError{Code: 451, EnhancedCode: smtp.EnhancedCode{4, 3, 0}, Message: "temporary failure"}
	}
}

// go-smtp advertises STARTTLS automatically once TLSConfig is set.
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

	switch strings.ToLower(strings.TrimSpace(cfg.TLSMode)) {
	case "", "none":
		// plain SMTP; STARTTLS not advertised
	case "starttls":
		cert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
		if err != nil {
			return nil, fmt.Errorf("load inbound TLS cert: %w", err)
		}
		srv.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			ServerName:   cfg.Hostname,
			MinVersion:   tls.VersionTLS12,
		}
	default:
		return nil, fmt.Errorf("unsupported inbound TLS mode %q (use none or starttls)", cfg.TLSMode)
	}
	return srv, nil
}

func extractDomain(addr string) string {
	at := strings.LastIndex(addr, "@")
	if at < 0 || at == len(addr)-1 {
		return ""
	}
	return strings.ToLower(addr[at+1:])
}
