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

package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// SettingsProvider is an interface for retrieving dynamic rate limit settings.
type SettingsProvider interface {
	DefaultRateLimitHourly() int
	DefaultRateLimitDaily() int
	LoginRateLimitCount() int
	LoginRateLimitWindowMinutes() int
}

type RedisLimiter struct {
	client      *redis.Client
	hourlyLimit int
	dailyLimit  int
	settings    SettingsProvider
}

func NewRedisLimiter(client *redis.Client, hourlyLimit, dailyLimit int) *RedisLimiter {
	return &RedisLimiter{
		client:      client,
		hourlyLimit: hourlyLimit,
		dailyLimit:  dailyLimit,
	}
}

// SetSettings sets a dynamic settings provider. When set, rate limits
// are read from the provider instead of using the static config values.
func (l *RedisLimiter) SetSettings(sp SettingsProvider) {
	l.settings = sp
}

func (l *RedisLimiter) effectiveLimits() (int, int) {
	if l.settings != nil {
		return l.settings.DefaultRateLimitHourly(), l.settings.DefaultRateLimitDaily()
	}
	return l.hourlyLimit, l.dailyLimit
}

// Allow checks if the user is within rate limits. Returns an error if exceeded.
func (l *RedisLimiter) Allow(ctx context.Context, userEmail string) error {
	hourlyLimit, dailyLimit := l.effectiveLimits()

	hourKey := fmt.Sprintf("ratelimit:hour:%s:%s", userEmail, time.Now().Format("2006010215"))
	dayKey := fmt.Sprintf("ratelimit:day:%s:%s", userEmail, time.Now().Format("20060102"))

	hourCount, err := l.client.Incr(ctx, hourKey).Result()
	if err != nil {
		return fmt.Errorf("rate limit check failed: %w", err)
	}
	if hourCount == 1 {
		l.client.Expire(ctx, hourKey, time.Hour)
	}
	if hourCount > int64(hourlyLimit) {
		return fmt.Errorf("hourly rate limit exceeded (%d/%d)", hourCount, hourlyLimit)
	}

	dayCount, err := l.client.Incr(ctx, dayKey).Result()
	if err != nil {
		return fmt.Errorf("rate limit check failed: %w", err)
	}
	if dayCount == 1 {
		l.client.Expire(ctx, dayKey, 24*time.Hour)
	}
	if dayCount > int64(dailyLimit) {
		return fmt.Errorf("daily rate limit exceeded (%d/%d)", dayCount, dailyLimit)
	}

	return nil
}

// Check verifies the user is within rate limits without incrementing the counters.
func (l *RedisLimiter) Check(ctx context.Context, userEmail string) error {
	hourlyLimit, dailyLimit := l.effectiveLimits()

	hourKey := fmt.Sprintf("ratelimit:hour:%s:%s", userEmail, time.Now().Format("2006010215"))
	dayKey := fmt.Sprintf("ratelimit:day:%s:%s", userEmail, time.Now().Format("20060102"))

	hourCount, err := l.client.Get(ctx, hourKey).Int64()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("rate limit check failed: %w", err)
	}
	if hourCount >= int64(hourlyLimit) {
		return fmt.Errorf("hourly rate limit exceeded (%d/%d)", hourCount, hourlyLimit)
	}

	dayCount, err := l.client.Get(ctx, dayKey).Int64()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("rate limit check failed: %w", err)
	}
	if dayCount >= int64(dailyLimit) {
		return fmt.Errorf("daily rate limit exceeded (%d/%d)", dayCount, dailyLimit)
	}

	return nil
}

// AllowLogin checks if a login attempt from the given IP is within rate limits.
// The max attempts and window duration are configurable via admin settings.
func (l *RedisLimiter) AllowLogin(ctx context.Context, ip string) error {
	maxAttempts := 10
	windowMinutes := 15
	if l.settings != nil {
		maxAttempts = l.settings.LoginRateLimitCount()
		windowMinutes = l.settings.LoginRateLimitWindowMinutes()
	}

	window := time.Duration(windowMinutes) * time.Minute
	// Bucket key: truncate current minute to the window size
	bucket := time.Now().Unix() / int64(windowMinutes*60)
	key := fmt.Sprintf("ratelimit:login:%s:%d", ip, bucket)

	count, err := l.client.Incr(ctx, key).Result()
	if err != nil {
		return nil
	}
	if count == 1 {
		l.client.Expire(ctx, key, window)
	}
	if count > int64(maxAttempts) {
		return fmt.Errorf("too many login attempts, try again later")
	}
	return nil
}
