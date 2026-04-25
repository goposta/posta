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

package inbound

import (
	"sync"
	"time"
)

// IPRateLimiter is a minimal in-process rate limiter keyed by remote IP. It is
// intentionally simple: a fixed-window counter with periodic eviction. Used to
// shed abusive SMTP clients before they can incur parsing cost.
type IPRateLimiter struct {
	mu       sync.Mutex
	window   time.Duration
	maxHits  int
	hits     map[string]*ipBucket
	lastEvct time.Time
}

type ipBucket struct {
	count int
	reset time.Time
}

// NewIPRateLimiter returns a limiter that allows up to maxHits per window per IP.
// maxHits ≤ 0 disables limiting.
func NewIPRateLimiter(maxHits int, window time.Duration) *IPRateLimiter {
	return &IPRateLimiter{
		window:  window,
		maxHits: maxHits,
		hits:    make(map[string]*ipBucket),
	}
}

// Allow returns true if the given IP is under its budget and increments the
// counter. Disabled limiters always return true.
func (l *IPRateLimiter) Allow(ip string) bool {
	if l == nil || l.maxHits <= 0 {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	if now.Sub(l.lastEvct) > l.window {
		l.evictLocked(now)
		l.lastEvct = now
	}

	b, ok := l.hits[ip]
	if !ok || now.After(b.reset) {
		l.hits[ip] = &ipBucket{count: 1, reset: now.Add(l.window)}
		return true
	}
	if b.count >= l.maxHits {
		return false
	}
	b.count++
	return true
}

func (l *IPRateLimiter) evictLocked(now time.Time) {
	for k, b := range l.hits {
		if now.After(b.reset) {
			delete(l.hits, k)
		}
	}
}
