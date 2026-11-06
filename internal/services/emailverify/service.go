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

// Package emailverify issues, delivers and redeems email verification tokens.
package emailverify

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/notification"
	"github.com/goposta/posta/internal/storage/repositories"
)

const (
	tokenTTL          = 24 * time.Hour
	resendWindow      = 1 * time.Hour
	resendMaxInWindow = 5
)

type Service struct {
	userRepo *repositories.UserRepository
	verRepo  *repositories.UserEmailVerificationRepository
	notifier *notification.Service
	appURL   string
	required bool
}

func NewService(
	userRepo *repositories.UserRepository,
	verRepo *repositories.UserEmailVerificationRepository,
	notifier *notification.Service,
	appURL string,
	required bool,
) *Service {
	return &Service{
		userRepo: userRepo,
		verRepo:  verRepo,
		notifier: notifier,
		appURL:   strings.TrimRight(appURL, "/"),
		required: required,
	}
}

// Required is false without a notifier, since the verification email cannot be delivered.
func (s *Service) Required() bool {
	if s == nil {
		return false
	}
	if s.notifier == nil || !s.notifier.IsConfigured() {
		return false
	}
	return s.required
}

func (s *Service) IsUserVerified(user *models.User) bool {
	if !s.Required() {
		return true
	}
	return user != nil && user.EmailVerifiedAt != nil
}

// IssueAndSend invalidates prior pending tokens and no-ops when the notifier is not configured.
func (s *Service) IssueAndSend(user *models.User) error {
	if s == nil || s.notifier == nil || !s.notifier.IsConfigured() {
		return nil
	}
	if user == nil {
		return errors.New("emailverify: nil user")
	}

	if err := s.verRepo.InvalidatePending(user.ID); err != nil {
		return fmt.Errorf("emailverify: invalidate pending: %w", err)
	}

	rawToken, hash, err := newToken()
	if err != nil {
		return err
	}
	v := &models.UserEmailVerification{
		UserID:    user.ID,
		TokenHash: hash,
		ExpiresAt: time.Now().Add(tokenTTL),
		CreatedAt: time.Now(),
	}
	if err := s.verRepo.Create(v); err != nil {
		return fmt.Errorf("emailverify: create token: %w", err)
	}

	verifyURL := fmt.Sprintf("%s/auth/verify-email?token=%s", s.appURL, rawToken)
	return s.notifier.Send(user.Email, "Verify your email address", notification.TemplateEmailVerify, map[string]any{
		"UserName":    displayName(user),
		"VerifyURL":   verifyURL,
		"ExpiryHours": int(tokenTTL.Hours()),
	})
}

func (s *Service) Redeem(rawToken string) (*models.User, bool, error) {
	if rawToken == "" {
		return nil, false, errors.New("token is required")
	}
	hash := hashToken(rawToken)
	v, err := s.verRepo.FindByTokenHash(hash)
	if err != nil {
		return nil, false, errors.New("invalid or expired token")
	}
	if v.UsedAt != nil {
		return nil, false, errors.New("token has already been used")
	}
	if time.Now().After(v.ExpiresAt) {
		return nil, false, errors.New("token has expired")
	}

	user, err := s.userRepo.FindByID(v.UserID)
	if err != nil {
		return nil, false, errors.New("user not found")
	}

	newlyVerified := user.EmailVerifiedAt == nil
	if newlyVerified {
		now := time.Now()
		user.EmailVerifiedAt = &now
		if err := s.userRepo.Update(user); err != nil {
			return nil, false, fmt.Errorf("emailverify: mark user verified: %w", err)
		}
	}
	if err := s.verRepo.MarkUsed(v.ID); err != nil {
		return nil, false, fmt.Errorf("emailverify: mark used: %w", err)
	}
	return user, newlyVerified, nil
}

func (s *Service) CanResend(userID uint) error {
	count, err := s.verRepo.CountRecentByUser(userID, time.Now().Add(-resendWindow))
	if err != nil {
		return nil
	}
	if count >= resendMaxInWindow {
		return fmt.Errorf("too many verification emails sent recently, try again later")
	}
	return nil
}

// MarkVerifiedNow is used for OAuth signups and when the notifier is not configured.
func (s *Service) MarkVerifiedNow(user *models.User) error {
	if user == nil || user.EmailVerifiedAt != nil {
		return nil
	}
	now := time.Now()
	user.EmailVerifiedAt = &now
	return s.userRepo.Update(user)
}

func newToken() (raw, hashHex string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	raw = hex.EncodeToString(buf)
	return raw, hashToken(raw), nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func displayName(u *models.User) string {
	if u.Name != "" {
		return u.Name
	}
	return u.Email
}
