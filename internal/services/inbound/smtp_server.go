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

// SMTPConfig holds configuration for the built-in SMTP receiver.
type SMTPConfig struct {
	Host           string
	Port           int
	Hostname       string
	MaxMessageSize int64
	TLSMode        string // "none" or "starttls"
	TLSCertFile    string
	TLSKeyFile     string
}

// Backend implements smtp.Backend for go-smtp.
type Backend struct {
	svc            *Service
	domainRepo     *repositories.DomainRepository
	maxMessageSize int64
	limiter        *IPRateLimiter
}

func NewBackend(svc *Service, domainRepo *repositories.DomainRepository, maxMessageSize int64) *Backend {
	return &Backend{svc: svc, domainRepo: domainRepo, maxMessageSize: maxMessageSize}
}

// SetRateLimiter enables per-IP rate limiting for new SMTP sessions.
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

// Rcpt rejects recipients whose domain is not ownership-verified, saving bandwidth
// by failing before DATA.
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

// Data reads the raw message (bounded by MaxMessageSize), parses it, and delegates
// to the service. Maps typed service errors back to SMTP status codes.
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

	parsed, err := ParseRawEmail(raw)
	if err != nil {
		logger.Warn("smtp parse error", "remote", s.remote, "error", err)
		return &smtp.SMTPError{Code: 550, EnhancedCode: smtp.EnhancedCode{5, 6, 0}, Message: "malformed message"}
	}
	if parsed.From == "" && s.from != "" {
		parsed.From = s.from
	}
	if len(parsed.To) == 0 {
		parsed.To = append([]string(nil), s.to...)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, ierr := s.backend.svc.Ingest(ctx, parsed, models.InboundSourceSMTP)
	switch {
	case ierr == nil, errors.Is(ierr, ErrDuplicate):
		return nil
	case errors.Is(ierr, ErrUnverifiedDomain):
		return &smtp.SMTPError{Code: 550, EnhancedCode: smtp.EnhancedCode{5, 1, 1}, Message: "relay not permitted"}
	case errors.Is(ierr, ErrSizeExceeded), errors.Is(ierr, ErrAttachmentTooLarge):
		return &smtp.SMTPError{Code: 552, EnhancedCode: smtp.EnhancedCode{5, 3, 4}, Message: "message too large"}
	case errors.Is(ierr, ErrSenderSuppressed):
		return &smtp.SMTPError{Code: 550, EnhancedCode: smtp.EnhancedCode{5, 7, 1}, Message: "sender suppressed"}
	default:
		logger.Error("inbound ingest failed", "remote", s.remote, "error", ierr)
		return &smtp.SMTPError{Code: 451, EnhancedCode: smtp.EnhancedCode{4, 3, 0}, Message: "temporary failure"}
	}
}

// NewSMTPServer builds a configured *smtp.Server. When cfg.TLSMode == "starttls",
// STARTTLS is automatically advertised by go-smtp once TLSConfig is set.
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
