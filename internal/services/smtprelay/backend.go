// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package smtprelay

import (
	"context"
	"encoding/base64"
	"io"
	"net"
	"net/mail"
	"strings"
	"time"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
	"github.com/goposta/posta/internal/middlewares"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/email"
	"github.com/goposta/posta/internal/services/inbound"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/logger"
)

type Backend struct {
	credRepo       *repositories.SMTPCredentialRepository
	userRepo       *repositories.UserRepository
	emailSvc       *email.Service
	maxMessageSize int64
	limiter        *inbound.IPRateLimiter
}

func NewBackend(credRepo *repositories.SMTPCredentialRepository, userRepo *repositories.UserRepository, emailSvc *email.Service, maxMessageSize int64) *Backend {
	return &Backend{
		credRepo:       credRepo,
		userRepo:       userRepo,
		emailSvc:       emailSvc,
		maxMessageSize: maxMessageSize,
	}
}

// SetRateLimiter enables per-IP rate limiting for new SMTP sessions.
func (b *Backend) SetRateLimiter(l *inbound.IPRateLimiter) { b.limiter = l }

func (b *Backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	remote := c.Conn().RemoteAddr().String()
	if b.limiter != nil {
		ip := remote
		if host, _, err := net.SplitHostPort(remote); err == nil {
			ip = host
		}
		if !b.limiter.Allow(ip) {
			logger.Warn("smtp relay: rate-limited remote", "ip", ip)
			return nil, &smtp.SMTPError{Code: 421, EnhancedCode: smtp.EnhancedCode{4, 7, 0}, Message: "rate limit exceeded"}
		}
	}
	return &session{
		backend: b,
		remote:  remote,
	}, nil
}

// credential stays non-nil for the connection's lifetime once AUTH succeeds; Reset does not clear it.
type session struct {
	backend    *Backend
	remote     string
	credential *models.SMTPCredential
	ownerEmail string
	from       string
	to         []string
}

func (s *session) AuthMechanisms() []string { return []string{sasl.Plain} }

func (s *session) Auth(mech string) (sasl.Server, error) {
	if mech != sasl.Plain {
		return nil, smtp.ErrAuthUnknownMechanism
	}
	return sasl.NewPlainServer(func(identity, username, password string) error {
		cred, err := s.backend.credRepo.FindByUsername(username)
		if err != nil || cred == nil || !cred.IsValid() {
			return smtp.ErrAuthFailed
		}
		if !VerifyPassword(cred, password) {
			return smtp.ErrAuthFailed
		}
		ip := s.remote
		if host, _, err := net.SplitHostPort(s.remote); err == nil {
			ip = host
		}
		if !middlewares.IPAllowed(cred.AllowedIPs, ip) {
			return smtp.ErrAuthFailed
		}
		owner, err := s.backend.userRepo.FindByID(cred.UserID)
		if err != nil {
			return smtp.ErrAuthFailed
		}

		s.credential = cred
		s.ownerEmail = owner.Email
		go func() { _ = s.backend.credRepo.UpdateLastUsed(cred.ID) }()
		return nil
	}), nil
}

func (s *session) Reset() {
	s.from = ""
	s.to = nil
}

func (s *session) Logout() error { return nil }

func (s *session) Mail(from string, _ *smtp.MailOptions) error {
	if s.credential == nil {
		return smtp.ErrAuthRequired
	}
	if from != "" {
		if addr, err := mail.ParseAddress(from); err == nil {
			s.from = addr.Address
		} else {
			s.from = from
		}
	}
	return nil
}

func (s *session) Rcpt(to string, _ *smtp.RcptOptions) error {
	if s.credential == nil {
		return smtp.ErrAuthRequired
	}
	addr := to
	if parsed, err := mail.ParseAddress(to); err == nil {
		addr = parsed.Address
	}
	s.to = append(s.to, addr)
	return nil
}

// Data reads the raw message (bounded by MaxMessageSize), parses it, and
// relays it through the outbound email pipeline using the SMTP envelope
// (MAIL FROM / RCPT TO) rather than the parsed header addresses, matching
// normal MTA behavior.
func (s *session) Data(r io.Reader) error {
	if s.credential == nil {
		return smtp.ErrAuthRequired
	}

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

	parsed, perr := inbound.ParseRawEmail(raw)
	if perr != nil {
		return &smtp.SMTPError{Code: 554, EnhancedCode: smtp.EnhancedCode{5, 6, 0}, Message: "malformed message"}
	}

	attachments := make([]models.Attachment, 0, len(parsed.Attachments))
	for _, a := range parsed.Attachments {
		attachments = append(attachments, models.Attachment{
			Filename:    a.Filename,
			ContentType: a.ContentType,
			Content:     base64.StdEncoding.EncodeToString(a.Content),
		})
	}

	req := &email.SendRequest{
		From:        s.from,
		To:          s.to,
		Subject:     parsed.Subject,
		HTML:        parsed.HTMLBody,
		Text:        parsed.TextBody,
		Attachments: attachments,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, serr := s.backend.emailSvc.Send(ctx, s.credential.UserID, 0, &s.credential.WorkspaceID, s.ownerEmail, req)
	return mapSendError(serr, s.remote)
}

// mapSendError translates an error returned by email.Service.Send into the
// appropriate SMTP status for the submitting client.
func mapSendError(err error, remote string) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	switch {
	case strings.HasPrefix(msg, "rate_limit:"):
		return &smtp.SMTPError{Code: 452, EnhancedCode: smtp.EnhancedCode{4, 7, 0}, Message: "rate limit exceeded, try again later"}
	case strings.HasPrefix(msg, "domain_verification:"):
		return &smtp.SMTPError{Code: 550, EnhancedCode: smtp.EnhancedCode{5, 7, 1}, Message: "sender domain not verified for this workspace"}
	default:
		logger.Error("smtp relay: send failed", "remote", remote, "error", err)
		return &smtp.SMTPError{Code: 451, EnhancedCode: smtp.EnhancedCode{4, 3, 0}, Message: "temporary failure"}
	}
}
