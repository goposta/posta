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

package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/jkaninda/posta/internal/models"
	"github.com/jkaninda/posta/internal/storage/repositories"
)

const apiKeyPrefix = "psk_"

type APIKeyService struct {
	repo *repositories.APIKeyRepository
}

func NewAPIKeyService(repo *repositories.APIKeyRepository) *APIKeyService {
	return &APIKeyService{repo: repo}
}

// GenerateKey creates a new API key and returns the raw key (only shown once).
// If expiresAt is nil the key never expires.
func (s *APIKeyService) GenerateKey(userID uint, name string, allowedIPs []string, expiresAt *time.Time) (string, *models.APIKey, error) {
	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		return "", nil, fmt.Errorf("failed to generate key: %w", err)
	}

	rawKey := apiKeyPrefix + hex.EncodeToString(rawBytes)
	hash := hashKey(rawKey)

	key := &models.APIKey{
		UserID:     userID,
		Name:       name,
		KeyHash:    hash,
		KeyPrefix:  rawKey[:len(apiKeyPrefix)+8],
		AllowedIPs: allowedIPs,
		ExpiresAt:  expiresAt,
	}

	if err := s.repo.Create(key); err != nil {
		return "", nil, err
	}

	return rawKey, key, nil
}

// ValidateKey checks if a raw API key is valid and returns the matching APIKey.
func (s *APIKeyService) ValidateKey(rawKey string) (*models.APIKey, error) {
	if !strings.HasPrefix(rawKey, apiKeyPrefix) {
		return nil, fmt.Errorf("invalid key format")
	}

	prefix := rawKey[:len(apiKeyPrefix)+8]
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
