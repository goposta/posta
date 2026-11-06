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

package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	DashboardStatsTTL = 30 * time.Second
	AdminMetricsTTL   = 30 * time.Second
	UserMetricsTTL    = 30 * time.Second
	AnalyticsTTL      = 60 * time.Second

	prefixDashboard          = "cache:dashboard:"
	prefixAdminMetrics       = "cache:admin:metrics"
	prefixUserMetrics        = "cache:admin:user_metrics:"
	prefixUserAnalytics      = "cache:analytics:user:"
	prefixAdminAnalytics     = "cache:analytics:admin"
	prefixDashAnalytics      = "cache:dash_analytics:user:"
	prefixAdminDashAnalytics = "cache:dash_analytics:admin"
	prefixUserProvider       = "cache:provider:user:"
	prefixAdminProvider      = "cache:provider:admin"
)

type Cache struct {
	client *redis.Client
}

func New(client *redis.Client) *Cache {
	return &Cache{client: client}
}

func (c *Cache) Get(ctx context.Context, key string, dest any) bool {
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		return false
	}
	return json.Unmarshal(data, dest) == nil
}

func (c *Cache) Set(ctx context.Context, key string, value any, ttl time.Duration) {
	data, err := json.Marshal(value)
	if err != nil {
		return
	}
	c.client.Set(ctx, key, data, ttl)
}

func (c *Cache) Delete(ctx context.Context, key string) {
	c.client.Del(ctx, key)
}

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

func UserProviderBreakdownKey(userID int, from, to string) string {
	return fmt.Sprintf("%s%d:%s:%s", prefixUserProvider, userID, from, to)
}

func AdminProviderBreakdownKey(from, to string) string {
	return fmt.Sprintf("%s:%s:%s", prefixAdminProvider, from, to)
}

func (c *Cache) InvalidateUser(ctx context.Context, userID int) {
	c.Delete(ctx, DashboardKey(userID))
	c.Delete(ctx, UserMetricsKey(userID))
	// Per-user analytics (date-range queries include user ID in key)
	c.InvalidateByPattern(ctx, fmt.Sprintf("%s%d:*", prefixUserAnalytics, userID))
	c.InvalidateByPattern(ctx, fmt.Sprintf("%s%d:*", prefixDashAnalytics, userID))
	c.InvalidateByPattern(ctx, fmt.Sprintf("%s%d:*", prefixUserProvider, userID))
	// Admin-level caches are affected since user data rolls up into them.
	c.Delete(ctx, AdminMetricsKey())
}

func (c *Cache) InvalidateAdmin(ctx context.Context) {
	c.Delete(ctx, AdminMetricsKey())
}

// InvalidateByPattern uses SCAN so it never blocks Redis.
func (c *Cache) InvalidateByPattern(ctx context.Context, pattern string) {
	iter := c.client.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		c.client.Del(ctx, iter.Val())
	}
}

func (c *Cache) InvalidateAnalytics(ctx context.Context) {
	c.InvalidateByPattern(ctx, "cache:analytics:*")
	c.InvalidateByPattern(ctx, "cache:dash_analytics:*")
	c.InvalidateByPattern(ctx, "cache:provider:*")
}

func (c *Cache) InvalidateAll(ctx context.Context) {
	c.InvalidateByPattern(ctx, "cache:*")
}
