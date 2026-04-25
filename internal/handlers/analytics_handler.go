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

	"github.com/goposta/posta/internal/services/cache"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
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
	scope := getScope(c)
	ctx := c.Request().Context()

	// Try cache first
	scopeKey := int(scope.UserID)
	if scope.WorkspaceID != nil {
		scopeKey = int(*scope.WorkspaceID) + 1000000
	}
	cacheKey := cache.UserAnalyticsKey(scopeKey, req.From, req.To, req.Status)
	var resp AnalyticsResponse
	if h.cache.Get(ctx, cacheKey, &resp) {
		return ok(c, resp)
	}

	from, to := parseTimeRange(req.From, req.To)

	var daily []repositories.DailyCount
	var breakdown []repositories.StatusBreakdown
	var err error

	if scope.WorkspaceID != nil {
		daily, err = h.repo.WorkspaceDailyCounts(*scope.WorkspaceID, from, to, req.Status)
		if err != nil {
			return c.AbortInternalServerError("failed to fetch analytics")
		}
		breakdown, err = h.repo.WorkspaceStatusBreakdown(*scope.WorkspaceID, from, to)
	} else {
		daily, err = h.repo.DailyCounts(scope.UserID, from, to, req.Status)
		if err != nil {
			return c.AbortInternalServerError("failed to fetch analytics")
		}
		breakdown, err = h.repo.StatusBreakdown(scope.UserID, from, to)
	}
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
	scope := getScope(c)
	ctx := c.Request().Context()

	scopeKey := int(scope.UserID)
	if scope.WorkspaceID != nil {
		scopeKey = int(*scope.WorkspaceID) + 1000000
	}
	cacheKey := cache.DashboardAnalyticsKey(scopeKey, req.From, req.To)
	var resp DashboardAnalyticsResponse
	if h.cache.Get(ctx, cacheKey, &resp) {
		return ok(c, resp)
	}

	from, to := parseTimeRange(req.From, req.To)

	var delivery []repositories.DeliveryRatePoint
	var bouncePoints []repositories.BounceRatePoint
	var latency *repositories.LatencyPercentiles
	var err error

	if scope.WorkspaceID != nil {
		delivery, err = h.repo.WorkspaceDeliveryRateTrends(*scope.WorkspaceID, from, to)
		if err != nil {
			return c.AbortInternalServerError("failed to fetch delivery rate trends")
		}
		bouncePoints, err = h.repo.WorkspaceBounceRateTrends(*scope.WorkspaceID, from, to)
		if err != nil {
			return c.AbortInternalServerError("failed to fetch bounce rate trends")
		}
		latency, err = h.repo.WorkspaceLatencyPercentiles(*scope.WorkspaceID, from, to)
	} else {
		delivery, err = h.repo.DeliveryRateTrends(scope.UserID, from, to)
		if err != nil {
			return c.AbortInternalServerError("failed to fetch delivery rate trends")
		}
		bouncePoints, err = h.repo.BounceRateTrends(scope.UserID, from, to)
		if err != nil {
			return c.AbortInternalServerError("failed to fetch bounce rate trends")
		}
		latency, err = h.repo.LatencyPercentilesForUser(scope.UserID, from, to)
	}
	if err != nil {
		return c.AbortInternalServerError("failed to fetch latency percentiles")
	}

	resp = DashboardAnalyticsResponse{
		DeliveryRateTrends: delivery,
		BounceRateTrends:   bouncePoints,
		LatencyPercentiles: latency,
	}

	h.cache.Set(ctx, cacheKey, resp, cache.AnalyticsTTL)
	return ok(c, resp)
}

type ProviderBreakdownRequest struct {
	From string `query:"from"`
	To   string `query:"to"`
}

type ProviderBreakdownResponse struct {
	Providers []repositories.ProviderBreakdownPoint `json:"providers"`
}

// UserProviderBreakdown returns delivery counts grouped by recipient mailbox
// provider (Gmail, Outlook, Yahoo, ...) for the authenticated scope.
func (h *AnalyticsHandler) UserProviderBreakdown(c *okapi.Context, req *ProviderBreakdownRequest) error {
	scope := getScope(c)
	ctx := c.Request().Context()

	scopeKey := int(scope.UserID)
	if scope.WorkspaceID != nil {
		scopeKey = int(*scope.WorkspaceID) + 1000000
	}
	cacheKey := cache.UserProviderBreakdownKey(scopeKey, req.From, req.To)
	var resp ProviderBreakdownResponse
	if h.cache.Get(ctx, cacheKey, &resp) {
		return ok(c, resp)
	}

	from, to := parseTimeRange(req.From, req.To)

	var rows []repositories.ProviderBreakdownPoint
	var err error
	if scope.WorkspaceID != nil {
		rows, err = h.repo.WorkspaceProviderBreakdown(*scope.WorkspaceID, from, to)
	} else {
		rows, err = h.repo.ProviderBreakdown(scope.UserID, from, to)
	}
	if err != nil {
		return c.AbortInternalServerError("failed to fetch provider breakdown")
	}

	resp = ProviderBreakdownResponse{Providers: rows}
	h.cache.Set(ctx, cacheKey, resp, cache.AnalyticsTTL)
	return ok(c, resp)
}

// AdminProviderBreakdown returns provider breakdown across all users.
func (h *AnalyticsHandler) AdminProviderBreakdown(c *okapi.Context, req *ProviderBreakdownRequest) error {
	ctx := c.Request().Context()

	cacheKey := cache.AdminProviderBreakdownKey(req.From, req.To)
	var resp ProviderBreakdownResponse
	if h.cache.Get(ctx, cacheKey, &resp) {
		return ok(c, resp)
	}

	from, to := parseTimeRange(req.From, req.To)
	rows, err := h.repo.AdminProviderBreakdown(from, to)
	if err != nil {
		return c.AbortInternalServerError("failed to fetch provider breakdown")
	}

	resp = ProviderBreakdownResponse{Providers: rows}
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
