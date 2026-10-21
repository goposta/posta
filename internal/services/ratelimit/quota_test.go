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
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

type fakeCounter struct {
	counts map[string]int64
	calls  []string
	err    error
}

func (f *fakeCounter) Incr(_ context.Context, key string, _ time.Duration) (int64, error) {
	f.calls = append(f.calls, key)
	if f.err != nil {
		return 0, f.err
	}
	if f.counts == nil {
		f.counts = map[string]int64{}
	}
	f.counts[key]++
	return f.counts[key], nil
}

func TestQuotasAllowUpToLimit(t *testing.T) {
	q := NewQuotasWithCounter(&fakeCounter{})
	quota := []Quota{{Key: "k", Limit: 2, Window: time.Hour}}
	for i := 0; i < 2; i++ {
		if err := q.Check(context.Background(), quota); err != nil {
			t.Fatalf("hit %d: %v", i+1, err)
		}
	}
	if err := q.Check(context.Background(), quota); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("third hit = %v, want ErrQuotaExceeded", err)
	}
}

func TestQuotasStopAtFirstExceeded(t *testing.T) {
	counter := &fakeCounter{counts: map[string]int64{"ip": 5}}
	q := NewQuotasWithCounter(counter)
	err := q.Check(context.Background(), []Quota{
		{Key: "ip", Limit: 5, Window: time.Hour},
		{Key: "form", Limit: 100, Window: time.Hour},
	})
	if !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("err = %v", err)
	}
	if counter.counts["form"] != 0 {
		t.Fatal("a refused request must not be charged to later quotas")
	}
}

func TestQuotasFailClosed(t *testing.T) {
	q := NewQuotasWithCounter(&fakeCounter{err: errors.New("redis down")})
	err := q.Check(context.Background(), []Quota{{Key: "k", Limit: 10, Window: time.Hour}})
	if !errors.Is(err, ErrQuotaUnavailable) {
		t.Fatalf("err = %v, want ErrQuotaUnavailable", err)
	}
}

func TestQuotasSkipUnsetQuotas(t *testing.T) {
	counter := &fakeCounter{}
	q := NewQuotasWithCounter(counter)
	if err := q.Check(context.Background(), []Quota{
		{Key: "", Limit: 1, Window: time.Hour},
		{Key: "disabled", Limit: 0, Window: time.Hour},
	}); err != nil {
		t.Fatal(err)
	}
	if len(counter.calls) != 0 {
		t.Fatalf("unexpected counter calls: %v", counter.calls)
	}
}

// TestRedisCounterAlwaysSetsTTL runs only against a real Redis 7+, set via
// POSTA_TEST_REDIS_URL. It covers the bug this counter fixes: a key left
// without a TTL by a lost EXPIRE stayed over its limit forever.
func TestRedisCounterAlwaysSetsTTL(t *testing.T) {
	url := os.Getenv("POSTA_TEST_REDIS_URL")
	if url == "" {
		t.Skip("POSTA_TEST_REDIS_URL not set")
	}
	opt, err := redis.ParseURL(url)
	if err != nil {
		t.Fatal(err)
	}
	rdb := redis.NewClient(opt)
	defer func() { _ = rdb.Close() }()
	ctx := context.Background()
	key := "posta:test:quota:" + time.Now().Format("150405.000000")
	defer rdb.Del(ctx, key)

	// Simulate a key stranded by the old INCR-then-EXPIRE code.
	if err := rdb.Set(ctx, key, 50, 0).Err(); err != nil {
		t.Fatal(err)
	}
	c := redisCounter{rdb: rdb}
	n, err := c.Incr(ctx, key, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if n != 51 {
		t.Fatalf("count = %d, want 51", n)
	}
	ttl, err := rdb.TTL(ctx, key).Result()
	if err != nil {
		t.Fatal(err)
	}
	if ttl <= 0 || ttl > time.Minute {
		t.Fatalf("ttl = %v, want a TTL within the window", ttl)
	}

	// A later hit must not push the expiry out (fixed window, not sliding).
	time.Sleep(1100 * time.Millisecond)
	if _, err := c.Incr(ctx, key, time.Minute); err != nil {
		t.Fatal(err)
	}
	if ttl2, _ := rdb.TTL(ctx, key).Result(); ttl2 >= ttl {
		t.Fatalf("ttl grew from %v to %v; EXPIRE NX must not reset it", ttl, ttl2)
	}
}
