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

package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// Default TTLs for cached data.
	DashboardStatsTTL = 60 * time.Second
	AdminMetricsTTL   = 60 * time.Second
	UserMetricsTTL    = 60 * time.Second
	AnalyticsTTL      = 120 * time.Second

	prefixDashboard      = "cache:dashboard:"
	prefixAdminMetrics   = "cache:admin:metrics"
	prefixUserMetrics    = "cache:admin:user_metrics:"
	prefixUserAnalytics  = "cache:analytics:user:"
	prefixAdminAnalytics    = "cache:analytics:admin"
	prefixDashAnalytics     = "cache:dash_analytics:user:"
	prefixAdminDashAnalytics = "cache:dash_analytics:admin"
)

// Cache provides Redis-backed caching for dashboard stats and metrics.
type Cache struct {
	client *redis.Client
}

// New creates a new Cache backed by the given Redis client.
func New(client *redis.Client) *Cache {
	return &Cache{client: client}
}

// Get retrieves a cached value and unmarshals it into dest.
// Returns false if the key does not exist or on error.
func (c *Cache) Get(ctx context.Context, key string, dest any) bool {
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		return false
	}
	return json.Unmarshal(data, dest) == nil
}

// Set marshals value and stores it with the given TTL.
func (c *Cache) Set(ctx context.Context, key string, value any, ttl time.Duration) {
	data, err := json.Marshal(value)
	if err != nil {
		return
	}
	c.client.Set(ctx, key, data, ttl)
}

// Delete removes a cached key.
func (c *Cache) Delete(ctx context.Context, key string) {
	c.client.Del(ctx, key)
}

// --- Key builders ---

func DashboardKey(userID int) string {
	return fmt.Sprintf("%s%d", prefixDashboard, userID)
}

func AdminMetricsKey() string {
	return prefixAdminMetrics
}

func UserMetricsKey(userID int) string {
	return fmt.Sprintf("%s%d", prefixUserMetrics, userID)
}

func UserAnalyticsKey(userID int, from, to, status string) string {
	return fmt.Sprintf("%s%d:%s:%s:%s", prefixUserAnalytics, userID, from, to, status)
}

func AdminAnalyticsKey(from, to, status string) string {
	return fmt.Sprintf("%s:%s:%s:%s", prefixAdminAnalytics, from, to, status)
}

func DashboardAnalyticsKey(userID int, from, to string) string {
	return fmt.Sprintf("%s%d:%s:%s", prefixDashAnalytics, userID, from, to)
}

func AdminDashboardAnalyticsKey(from, to string) string {
	return fmt.Sprintf("%s:%s:%s", prefixAdminDashAnalytics, from, to)
}

// InvalidateUser removes all cached data scoped to a specific user.
func (c *Cache) InvalidateUser(ctx context.Context, userID int) {
	c.Delete(ctx, DashboardKey(userID))
	c.Delete(ctx, UserMetricsKey(userID))
	// Invalidate admin-level caches too, since user data affects them.
	c.Delete(ctx, AdminMetricsKey())
}

// InvalidateAdmin removes admin-level cached metrics.
func (c *Cache) InvalidateAdmin(ctx context.Context) {
	c.Delete(ctx, AdminMetricsKey())
}

// InvalidateByPattern removes keys matching a pattern using SCAN (non-blocking).
func (c *Cache) InvalidateByPattern(ctx context.Context, pattern string) {
	iter := c.client.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		c.client.Del(ctx, iter.Val())
	}
}

// InvalidateAnalytics removes all cached analytics data.
func (c *Cache) InvalidateAnalytics(ctx context.Context) {
	c.InvalidateByPattern(ctx, "cache:analytics:*")
}

// InvalidateAll removes all cache keys (dashboard + metrics + analytics).
func (c *Cache) InvalidateAll(ctx context.Context) {
	c.InvalidateByPattern(ctx, "cache:*")
}
