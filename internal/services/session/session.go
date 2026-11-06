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

package session

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const revokedPrefix = "session:revoked:"

// Store provides Redis-backed session revocation checking.
type Store struct {
	redis *redis.Client
}

func NewStore(client *redis.Client) *Store {
	return &Store{redis: client}
}

// MarkRevoked adds a JTI to the Redis blacklist with a TTL matching the token's remaining lifetime.
func (s *Store) MarkRevoked(ctx context.Context, jti string, expiresAt time.Time) {
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		return // already expired, no need to blacklist
	}
	s.redis.Set(ctx, revokedPrefix+jti, "1", ttl)
}

func (s *Store) IsRevoked(ctx context.Context, jti string) bool {
	val, err := s.redis.Exists(ctx, revokedPrefix+jti).Result()
	if err != nil {
		return false // fail open to avoid locking everyone out on Redis errors
	}
	return val > 0
}

func RevokedKey(jti string) string {
	return fmt.Sprintf("%s%s", revokedPrefix, jti)
}
