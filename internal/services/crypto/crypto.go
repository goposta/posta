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

// Init derives the AES key from secret; an empty secret disables encryption and leaves passwords base64-only.
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

func Enabled() bool {
	keyMu.RLock()
	defer keyMu.RUnlock()
	return key != nil
}

// Encrypt returns "enc:"+base64(nonce||ciphertext), or plain base64 when encryption is not configured.
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

// Decrypt treats values without the "enc:" prefix as legacy base64.
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

	decoded, err := base64.StdEncoding.DecodeString(stored)
	if err != nil {
		return stored, nil
	}
	return string(decoded), nil
}

func IsEncrypted(s string) bool {
	return strings.HasPrefix(s, encryptedPrefix)
}

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
