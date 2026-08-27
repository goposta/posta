// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package smtprelay

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/storage/repositories"
)

// SMTPUsernamePrefix identifies a generated SMTP Relay username.
const SMTPUsernamePrefix = "smtp_"

type CredentialService struct {
	repo *repositories.SMTPCredentialRepository
}

func NewCredentialService(repo *repositories.SMTPCredentialRepository) *CredentialService {
	return &CredentialService{repo: repo}
}

func (s *CredentialService) GenerateCredential(userID, workspaceID uint, name string, allowedIPs []string) (username, password string, cred *models.SMTPCredential, err error) {
	userBytes := make([]byte, 8)
	if _, err = rand.Read(userBytes); err != nil {
		return "", "", nil, fmt.Errorf("failed to generate username: %w", err)
	}
	username = SMTPUsernamePrefix + hex.EncodeToString(userBytes)

	passBytes := make([]byte, 32)
	if _, err = rand.Read(passBytes); err != nil {
		return "", "", nil, fmt.Errorf("failed to generate password: %w", err)
	}
	password = hex.EncodeToString(passBytes)

	cred = &models.SMTPCredential{
		WorkspaceID:  workspaceID,
		UserID:       userID,
		Name:         name,
		Username:     username,
		PasswordHash: hashPassword(password),
		AllowedIPs:   allowedIPs,
	}

	if err = s.repo.Create(cred); err != nil {
		return "", "", nil, err
	}

	return username, password, cred, nil
}

// VerifyPassword reports whether password matches cred's stored hash.
func VerifyPassword(cred *models.SMTPCredential, password string) bool {
	return cred.PasswordHash == hashPassword(password)
}

func hashPassword(password string) string {
	h := sha256.Sum256([]byte(password))
	return hex.EncodeToString(h[:])
}
