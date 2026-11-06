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

package messages

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ErrNonceInvalid = errors.New("invalid nonce")
	ErrNonceExpired = errors.New("nonce expired")
	ErrNonceReplay  = errors.New("nonce already used")
	ErrNonceTooFast = errors.New("submitted too quickly")
)

const nonceTTL = 30 * time.Minute

type Nonce struct {
	Value     string `json:"nonce"`
	ExpiresIn int    `json:"expires_in"`
}

// Nonces issues and checks the signed, single-use tokens that public
// endpoints require before accepting a submission. A nonce is bound to one
// public key, carries its issue time, and is remembered in Redis once used.
type Nonces struct {
	key          []byte
	redis        *redis.Client
	replayPrefix string
}

// NewNonces returns Nonces signed with key. replayPrefix namespaces the Redis
// keys that remember used nonces; without Redis, replays are not detected.
func NewNonces(key []byte, rdb *redis.Client, replayPrefix string) *Nonces {
	return &Nonces{key: key, redis: rdb, replayPrefix: replayPrefix}
}

func (s *Service) nonces() *Nonces { return NewNonces(s.hmacKey, s.redis, "form:nonce:") }

func (s *Service) IssueNonce(ctx context.Context, formKey string) (*Nonce, error) {
	return s.nonces().Issue(ctx, formKey)
}

func (s *Service) VerifyNonce(ctx context.Context, formKey, token string, minFillSeconds int) error {
	return s.nonces().Verify(ctx, formKey, token, minFillSeconds)
}

// Issue returns a nonce for the endpoint with the given public key.
func (n *Nonces) Issue(_ context.Context, publicKey string) (*Nonce, error) {
	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	issued := time.Now().Unix()
	body := fmt.Sprintf("%s.%d.%s", publicKey, issued, base64.RawURLEncoding.EncodeToString(raw))
	token := body + "." + sign(n.key, body)

	return &Nonce{Value: token, ExpiresIn: int(nonceTTL.Seconds())}, nil
}

// Verify checks a nonce for publicKey: its signature, its age, that at least
// minFillSeconds have passed since it was issued, and that it is unused.
func (n *Nonces) Verify(ctx context.Context, publicKey, token string, minFillSeconds int) error {
	parts := strings.Split(token, ".")
	if len(parts) != 4 {
		return ErrNonceInvalid
	}
	body := strings.Join(parts[:3], ".")
	if !hmac.Equal([]byte(sign(n.key, body)), []byte(parts[3])) {
		return ErrNonceInvalid
	}
	if parts[0] != publicKey {
		return ErrNonceInvalid
	}

	issued, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return ErrNonceInvalid
	}
	age := time.Since(time.Unix(issued, 0))
	if age > nonceTTL || age < 0 {
		return ErrNonceExpired
	}
	if minFillSeconds > 0 && age < time.Duration(minFillSeconds)*time.Second {
		return ErrNonceTooFast
	}

	if n.redis != nil {
		key := n.replayPrefix + sign(n.key, token)
		set, err := n.redis.SetNX(ctx, key, 1, nonceTTL).Result()
		if err == nil && !set {
			return ErrNonceReplay
		}
	}
	return nil
}

func sign(key []byte, body string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(body))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
