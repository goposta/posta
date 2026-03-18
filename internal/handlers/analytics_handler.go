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

package handlers

import (
	"time"

	"github.com/jkaninda/okapi"
	"github.com/jkaninda/posta/internal/services/cache"
	"github.com/jkaninda/posta/internal/storage/repositories"
)

type AnalyticsHandler struct {
	repo  *repositories.AnalyticsRepository
	cache *cache.Cache
}

func NewAnalyticsHandler(repo *repositories.AnalyticsRepository, c *cache.Cache) *AnalyticsHandler {
	return &AnalyticsHandler{repo: repo, cache: c}
}

type AnalyticsRequest struct {
	From   string `query:"from"`
	To     string `query:"to"`
	Status string `query:"status"`
}

type AnalyticsResponse struct {
	DailyCounts     []repositories.DailyCount      `json:"daily_counts"`
	StatusBreakdown []repositories.StatusBreakdown `json:"status_breakdown"`
}

type DashboardAnalyticsRequest struct {
	From string `query:"from"`
	To   string `query:"to"`
}

type DashboardAnalyticsResponse struct {
	DeliveryRateTrends []repositories.DeliveryRatePoint `json:"delivery_rate_trends"`
	BounceRateTrends   []repositories.BounceRatePoint   `json:"bounce_rate_trends"`
	LatencyPercentiles *repositories.LatencyPercentiles `json:"latency_percentiles"`
}

func (h *AnalyticsHandler) UserAnalytics(c *okapi.Context, req *AnalyticsRequest) error {
	userID := c.GetInt("user_id")
	ctx := c.Request().Context()

	// Try cache first
	cacheKey := cache.UserAnalyticsKey(userID, req.From, req.To, req.Status)
	var resp AnalyticsResponse
	if h.cache.Get(ctx, cacheKey, &resp) {
		return ok(c, resp)
	}

	from, to := parseTimeRange(req.From, req.To)

	daily, err := h.repo.DailyCounts(uint(userID), from, to, req.Status)
	if err != nil {
		return c.AbortInternalServerError("failed to fetch analytics")
	}
	breakdown, err := h.repo.StatusBreakdown(uint(userID), from, to)
	if err != nil {
		return c.AbortInternalServerError("failed to fetch analytics")
	}

	resp = AnalyticsResponse{
		DailyCounts:     daily,
		StatusBreakdown: breakdown,
	}

	h.cache.Set(ctx, cacheKey, resp, cache.AnalyticsTTL)

	return ok(c, resp)
}

func (h *AnalyticsHandler) AdminAnalytics(c *okapi.Context, req *AnalyticsRequest) error {
	ctx := c.Request().Context()

	// Try cache first
	cacheKey := cache.AdminAnalyticsKey(req.From, req.To, req.Status)
	var resp AnalyticsResponse
	if h.cache.Get(ctx, cacheKey, &resp) {
		return ok(c, resp)
	}

	from, to := parseTimeRange(req.From, req.To)

	daily, err := h.repo.AdminDailyCounts(from, to, req.Status)
	if err != nil {
		return c.AbortInternalServerError("failed to fetch analytics")
	}
	breakdown, err := h.repo.AdminStatusBreakdown(from, to)
	if err != nil {
		return c.AbortInternalServerError("failed to fetch analytics")
	}

	resp = AnalyticsResponse{
		DailyCounts:     daily,
		StatusBreakdown: breakdown,
	}

	h.cache.Set(ctx, cacheKey, resp, cache.AnalyticsTTL)

	return ok(c, resp)
}

func (h *AnalyticsHandler) UserDashboardAnalytics(c *okapi.Context, req *DashboardAnalyticsRequest) error {
	userID := c.GetInt("user_id")
	ctx := c.Request().Context()

	cacheKey := cache.DashboardAnalyticsKey(userID, req.From, req.To)
	var resp DashboardAnalyticsResponse
	if h.cache.Get(ctx, cacheKey, &resp) {
		return ok(c, resp)
	}

	from, to := parseTimeRange(req.From, req.To)

	delivery, err := h.repo.DeliveryRateTrends(uint(userID), from, to)
	if err != nil {
		return c.AbortInternalServerError("failed to fetch delivery rate trends")
	}
	bounces, err := h.repo.BounceRateTrends(uint(userID), from, to)
	if err != nil {
		return c.AbortInternalServerError("failed to fetch bounce rate trends")
	}
	latency, err := h.repo.LatencyPercentilesForUser(uint(userID), from, to)
	if err != nil {
		return c.AbortInternalServerError("failed to fetch latency percentiles")
	}

	resp = DashboardAnalyticsResponse{
		DeliveryRateTrends: delivery,
		BounceRateTrends:   bounces,
		LatencyPercentiles: latency,
	}

	h.cache.Set(ctx, cacheKey, resp, cache.AnalyticsTTL)
	return ok(c, resp)
}

func (h *AnalyticsHandler) AdminDashboardAnalytics(c *okapi.Context, req *DashboardAnalyticsRequest) error {
	ctx := c.Request().Context()

	cacheKey := cache.AdminDashboardAnalyticsKey(req.From, req.To)
	var resp DashboardAnalyticsResponse
	if h.cache.Get(ctx, cacheKey, &resp) {
		return ok(c, resp)
	}

	from, to := parseTimeRange(req.From, req.To)

	delivery, err := h.repo.AdminDeliveryRateTrends(from, to)
	if err != nil {
		return c.AbortInternalServerError("failed to fetch delivery rate trends")
	}
	bounces, err := h.repo.AdminBounceRateTrends(from, to)
	if err != nil {
		return c.AbortInternalServerError("failed to fetch bounce rate trends")
	}
	latency, err := h.repo.AdminLatencyPercentiles(from, to)
	if err != nil {
		return c.AbortInternalServerError("failed to fetch latency percentiles")
	}

	resp = DashboardAnalyticsResponse{
		DeliveryRateTrends: delivery,
		BounceRateTrends:   bounces,
		LatencyPercentiles: latency,
	}

	h.cache.Set(ctx, cacheKey, resp, cache.AnalyticsTTL)
	return ok(c, resp)
}

func parseTimeRange(fromStr, toStr string) (time.Time, time.Time) {
	to := time.Now()
	from := to.AddDate(0, 0, -30) // default: last 30 days

	if fromStr != "" {
		if t, err := time.Parse("2006-01-02", fromStr); err == nil {
			from = t
		}
	}
	if toStr != "" {
		if t, err := time.Parse("2006-01-02", toStr); err == nil {
			to = t.Add(24*time.Hour - time.Second) // end of day
		}
	}
	return from, to
}
