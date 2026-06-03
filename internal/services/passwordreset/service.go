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

// Package passwordreset handles the self-service "forgot password" flow: token
// issuance, delivery via a notification template, and redemption (which sets a
// new password). The feature is gated by a platform setting checked at the
// handler layer; this service is concerned only with token mechanics.
package passwordreset

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
	"golang.org/x/crypto/bcrypt"
)

const (
	tokenTTL          = 1 * time.Hour
	resendWindow      = 1 * time.Hour
	resendMaxInWindow = 5
)

// Service coordinates password-reset token issuance and redemption.
type Service struct {
	userRepo  *repositories.UserRepository
	tokenRepo *repositories.PasswordResetRepository
	notifier  *notification.Service
	appURL    string
}

func NewService(
	userRepo *repositories.UserRepository,
	tokenRepo *repositories.PasswordResetRepository,
	notifier *notification.Service,
	appURL string,
) *Service {
	return &Service{
		userRepo:  userRepo,
		tokenRepo: tokenRepo,
		notifier:  notifier,
		appURL:    strings.TrimRight(appURL, "/"),
	}
}

func (s *Service) Deliverable() bool {
	return s != nil && s.notifier != nil && s.notifier.IsConfigured()
}

func (s *Service) IssueAndSend(user *models.User) error {
	if !s.Deliverable() {
		return nil
	}
	if user == nil {
		return errors.New("passwordreset: nil user")
	}

	if err := s.tokenRepo.InvalidatePending(user.ID); err != nil {
		return fmt.Errorf("passwordreset: invalidate pending: %w", err)
	}

	rawToken, hash, err := newToken()
	if err != nil {
		return err
	}
	t := &models.PasswordResetToken{
		UserID:    user.ID,
		TokenHash: hash,
		ExpiresAt: time.Now().Add(tokenTTL),
		CreatedAt: time.Now(),
	}
	if err := s.tokenRepo.Create(t); err != nil {
		return fmt.Errorf("passwordreset: create token: %w", err)
	}

	resetURL := fmt.Sprintf("%s/auth/reset-password?token=%s", s.appURL, rawToken)
	return s.notifier.Send(user.Email, "Reset your password", notification.TemplatePasswordReset, map[string]any{
		"UserName":    displayName(user),
		"ResetURL":    resetURL,
		"ExpiryHours": int(tokenTTL.Hours()),
	})
}

func (s *Service) Redeem(rawToken, newPassword string) (*models.User, error) {
	if rawToken == "" {
		return nil, errors.New("token is required")
	}
	t, err := s.tokenRepo.FindByTokenHash(hashToken(rawToken))
	if err != nil {
		return nil, errors.New("invalid or expired token")
	}
	if t.UsedAt != nil {
		return nil, errors.New("this reset link has already been used")
	}
	if time.Now().After(t.ExpiresAt) {
		return nil, errors.New("this reset link has expired")
	}

	user, err := s.userRepo.FindByID(t.UserID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("passwordreset: hash password: %w", err)
	}
	user.PasswordHash = string(hash)
	if err := s.userRepo.Update(user); err != nil {
		return nil, fmt.Errorf("passwordreset: update password: %w", err)
	}

	if err := s.tokenRepo.MarkUsed(t.ID); err != nil {
		return nil, fmt.Errorf("passwordreset: mark used: %w", err)
	}
	// Burn any other outstanding tokens so a second link can't reset again.
	_ = s.tokenRepo.InvalidatePending(user.ID)
	return user, nil
}

// CanIssue returns an error when the user has hit the request rate limit.
func (s *Service) CanIssue(userID uint) error {
	count, err := s.tokenRepo.CountRecentByUser(userID, time.Now().Add(-resendWindow))
	if err != nil {
		return nil
	}
	if count >= resendMaxInWindow {
		return fmt.Errorf("too many password reset requests recently, try again later")
	}
	return nil
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
