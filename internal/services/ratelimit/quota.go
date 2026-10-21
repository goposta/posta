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

package ratelimit

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	// ErrQuotaExceeded means a counter went over its limit.
	ErrQuotaExceeded = errors.New("quota exceeded")
	// ErrQuotaUnavailable means a counter could not be read. Public endpoints
	// treat it as exceeded: one that stops accepting is better than one that
	// stops limiting.
	ErrQuotaUnavailable = errors.New("quota store unavailable")
)

// Quota is one fixed-window counter: at most Limit hits per Window under Key.
type Quota struct {
	Key    string
	Limit  int
	Window time.Duration
}

// Counter increments a key and guarantees it expires.
type Counter interface {
	Incr(ctx context.Context, key string, window time.Duration) (int64, error)
}

type redisCounter struct {
	rdb *redis.Client
}

// Incr runs INCR and EXPIRE NX in one MULTI, so a key can never be left
// without a TTL; a key stranded without one by older code is given one on its
// next hit. EXPIRE NX needs Redis 7.0 or later.
func (c redisCounter) Incr(ctx context.Context, key string, window time.Duration) (int64, error) {
	pipe := c.rdb.TxPipeline()
	incr := pipe.Incr(ctx, key)
	pipe.ExpireNX(ctx, key, window)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return incr.Val(), nil
}

// Quotas checks a set of counters for one request.
type Quotas struct {
	counter Counter
}

// NewQuotas returns Quotas backed by Redis.
func NewQuotas(rdb *redis.Client) *Quotas {
	return &Quotas{counter: redisCounter{rdb: rdb}}
}

// NewQuotasWithCounter returns Quotas over any Counter, for tests.
func NewQuotasWithCounter(c Counter) *Quotas {
	return &Quotas{counter: c}
}

// Check counts the request against each quota in order and stops at the first
// one it exceeds, so later counters are not charged for a request that was
// already refused. A quota with no key or a non-positive limit is skipped.
func (q *Quotas) Check(ctx context.Context, quotas []Quota) error {
	for _, quota := range quotas {
		if quota.Key == "" || quota.Limit <= 0 {
			continue
		}
		n, err := q.counter.Incr(ctx, quota.Key, quota.Window)
		if err != nil {
			return ErrQuotaUnavailable
		}
		if n > int64(quota.Limit) {
			return ErrQuotaExceeded
		}
	}
	return nil
}
