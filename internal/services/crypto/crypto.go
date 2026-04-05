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

package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
)

const encryptedPrefix = "enc:"

// key holds the derived 32-byte AES key. nil means encryption is disabled (base64 fallback).
var (
	key   []byte
	keyMu sync.RWMutex
)

// Init derives a 32-byte AES-256 key from the provided secret via SHA-256.
// Call this once at startup. If secret is empty, encryption is disabled and
// passwords will be stored with base64 encoding only.
func Init(secret string) {
	keyMu.Lock()
	defer keyMu.Unlock()
	if secret == "" {
		key = nil
		return
	}
	h := sha256.Sum256([]byte(secret))
	key = h[:]
}

// Enabled returns true if encryption is configured.
func Enabled() bool {
	keyMu.RLock()
	defer keyMu.RUnlock()
	return key != nil
}

// Encrypt encrypts plaintext with AES-256-GCM.
// Returns "enc:" + base64(nonce||ciphertext).
// If encryption is not configured, returns base64-encoded plaintext.
func Encrypt(plaintext string) (string, error) {
	keyMu.RLock()
	k := key
	keyMu.RUnlock()

	if k == nil {
		return base64.StdEncoding.EncodeToString([]byte(plaintext)), nil
	}

	ct, err := aesGCMEncrypt(k, []byte(plaintext))
	if err != nil {
		return "", fmt.Errorf("crypto: failed to encrypt: %w", err)
	}
	return encryptedPrefix + base64.StdEncoding.EncodeToString(ct), nil
}

// Decrypt decrypts a stored value. If it has the "enc:" prefix, AES-256-GCM
// decryption is used. Otherwise it is treated as base64.
func Decrypt(stored string) (string, error) {
	if IsEncrypted(stored) {
		keyMu.RLock()
		k := key
		keyMu.RUnlock()

		if k == nil {
			return "", fmt.Errorf("crypto: encrypted value found but no encryption key configured")
		}

		encoded := strings.TrimPrefix(stored, encryptedPrefix)
		ct, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return "", fmt.Errorf("crypto: failed to decode ciphertext: %w", err)
		}
		pt, err := aesGCMDecrypt(k, ct)
		if err != nil {
			return "", fmt.Errorf("crypto: failed to decrypt: %w", err)
		}
		return string(pt), nil
	}

	// base64 fallback
	decoded, err := base64.StdEncoding.DecodeString(stored)
	if err != nil {
		return stored, nil
	}
	return string(decoded), nil
}

// IsEncrypted returns true if the stored value uses the encrypted format.
func IsEncrypted(s string) bool {
	return strings.HasPrefix(s, encryptedPrefix)
}

// aesGCMEncrypt performs AES-256-GCM encryption, returning nonce||ciphertext.
func aesGCMEncrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// aesGCMDecrypt performs AES-256-GCM decryption on nonce||ciphertext.
func aesGCMDecrypt(key, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	return gcm.Open(nil, data[:nonceSize], data[nonceSize:], nil)
}
