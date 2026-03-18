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

// NewStore creates a new session store backed by Redis.
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

// IsRevoked checks if a JTI is in the Redis blacklist.
func (s *Store) IsRevoked(ctx context.Context, jti string) bool {
	val, err := s.redis.Exists(ctx, revokedPrefix+jti).Result()
	if err != nil {
		return false // fail open to avoid locking everyone out on Redis errors
	}
	return val > 0
}

// RevokedKey returns the Redis key for a revoked session.
func RevokedKey(jti string) string {
	return fmt.Sprintf("%s%s", revokedPrefix, jti)
}
