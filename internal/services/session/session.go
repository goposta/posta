/*
 *  MIT License
 *
 * Copyright (c) 2026 Jonas Kaninda
 *
 *  Permission is hereby granted, free of charge, to any person obtaining a copy
 *  of this software and associated documentation files (the "Software"), to deal
 *  in the Software without restriction, including without limitation the rights
 *  to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 *  copies of the Software, and to permit persons to whom the Software is
 *  furnished to do so, subject to the following conditions:
 *
 *  The above copyright notice and this permission notice shall be included in all
 *  copies or substantial portions of the Software.
 *
 *  THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 *  IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 *  FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 *  AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 *  LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 *  OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 *  SOFTWARE.
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
