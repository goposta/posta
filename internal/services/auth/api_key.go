// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/storage/repositories"
)

// APIKeyPrefix identifies a raw Posta API key. Auth middleware uses it to tell
// an API key apart from a JWT without parsing either.
const APIKeyPrefix = "psk_"

type APIKeyService struct {
	repo *repositories.APIKeyRepository
}

func NewAPIKeyService(repo *repositories.APIKeyRepository) *APIKeyService {
	return &APIKeyService{repo: repo}
}

func NormalizeScopes(scopes []string) ([]string, error) {
	if len(scopes) == 0 {
		return []string{models.ScopeSend}, nil
	}
	seen := make(map[string]bool, len(scopes))
	out := make([]string, 0, len(scopes))
	for _, sc := range scopes {
		if !models.ValidScopes[sc] {
			return nil, fmt.Errorf("unknown scope: %q", sc)
		}
		if seen[sc] {
			continue
		}
		seen[sc] = true
		out = append(out, sc)
	}
	return out, nil
}

func (s *APIKeyService) GenerateKey(userID uint, workspaceID *uint, name string, allowedIPs []string, scopes []string, expiresAt *time.Time) (string, *models.APIKey, error) {
	normScopes, err := NormalizeScopes(scopes)
	if err != nil {
		return "", nil, err
	}

	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		return "", nil, fmt.Errorf("failed to generate key: %w", err)
	}

	rawKey := APIKeyPrefix + hex.EncodeToString(rawBytes)
	hash := hashKey(rawKey)

	key := &models.APIKey{
		UserID:      userID,
		WorkspaceID: workspaceID,
		Name:        name,
		KeyHash:     hash,
		KeyPrefix:   rawKey[:len(APIKeyPrefix)+8],
		AllowedIPs:  allowedIPs,
		Scopes:      normScopes,
		ExpiresAt:   expiresAt,
	}

	if err := s.repo.Create(key); err != nil {
		return "", nil, err
	}

	return rawKey, key, nil
}

// ValidateKey checks if a raw API key is valid and returns the matching APIKey.
func (s *APIKeyService) ValidateKey(rawKey string) (*models.APIKey, error) {
	if !strings.HasPrefix(rawKey, APIKeyPrefix) {
		return nil, fmt.Errorf("invalid key format")
	}
	if len(rawKey) < len(APIKeyPrefix)+8 {
		return nil, fmt.Errorf("invalid key format")
	}

	prefix := rawKey[:len(APIKeyPrefix)+8]
	candidates, err := s.repo.FindByPrefix(prefix)
	if err != nil {
		return nil, fmt.Errorf("key not found")
	}

	hash := hashKey(rawKey)
	for i := range candidates {
		if candidates[i].KeyHash == hash {
			if !candidates[i].IsValid() {
				return nil, fmt.Errorf("key is expired or revoked")
			}
			return &candidates[i], nil
		}
	}

	return nil, fmt.Errorf("key not found")
}

func hashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}
