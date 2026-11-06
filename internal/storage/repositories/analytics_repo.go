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

package repositories

import (
	"sort"
	"time"

	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
)

type AnalyticsRepository struct {
	db *gorm.DB
}

func NewAnalyticsRepository(db *gorm.DB) *AnalyticsRepository {
	return &AnalyticsRepository{db: db}
}

type DailyCount struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

type StatusBreakdown struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

func (r *AnalyticsRepository) AdminDailyCounts(from, to time.Time, status string) ([]DailyCount, error) {
	var results []DailyCount
	query := r.db.Table("emails").
		Select("TO_CHAR(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD') as date, COUNT(*) as count").
		Where("created_at >= ? AND created_at <= ?", from, to)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	err := query.Group("date").Order("date ASC").Find(&results).Error
	return results, err
}

func (r *AnalyticsRepository) AdminStatusBreakdown(from, to time.Time) ([]StatusBreakdown, error) {
	var results []StatusBreakdown
	err := r.db.Table("emails").
		Select("status, COUNT(*) as count").
		Where("created_at >= ? AND created_at <= ?", from, to).
		Group("status").
		Find(&results).Error
	return results, err
}

type deliveryRow struct {
	Date   string
	Status string
	Count  int64
}

type bounceRow struct {
	Date  string
	Type  string
	Count int64
}

type DeliveryRatePoint struct {
	Date         string  `json:"date"`
	Sent         int64   `json:"sent"`
	Failed       int64   `json:"failed"`
	Total        int64   `json:"total"`
	DeliveryRate float64 `json:"delivery_rate"`
}

type BounceRatePoint struct {
	Date      string `json:"date"`
	Hard      int64  `json:"hard"`
	Soft      int64  `json:"soft"`
	Complaint int64  `json:"complaint"`
	Total     int64  `json:"total"`
}

type LatencyPercentiles struct {
	P50 float64 `json:"p50"`
	P75 float64 `json:"p75"`
	P90 float64 `json:"p90"`
	P99 float64 `json:"p99"`
	Avg float64 `json:"avg"`
}

func (r *AnalyticsRepository) AdminDeliveryRateTrends(from, to time.Time) ([]DeliveryRatePoint, error) {
	var rows []deliveryRow
	err := r.db.Table("emails").
		Select("TO_CHAR(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD') as date, status, COUNT(*) as count").
		Where("created_at >= ? AND created_at <= ? AND status IN ?", from, to, []string{string(models.EmailStatusSent), string(models.EmailStatusFailed)}).
		Group("date, status").Order("date ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return buildDeliveryRatePoints(rows, from, to), nil
}

// utcDay must match the queries' UTC buckets; time.Truncate rounds against zero time and loses a day west of UTC.
func utcDay(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

// dayKeys is gapless so a quiet day renders as a zero rather than vanishing from the axis.
func dayKeys(from, to time.Time) []string {
	start, end := utcDay(from), utcDay(to)
	if end.Before(start) {
		return nil
	}
	keys := make([]string, 0, int(end.Sub(start)/(24*time.Hour))+1)
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		keys = append(keys, d.Format("2006-01-02"))
	}
	return keys
}

// buildDeliveryRatePoints folds out-of-range rows onto the nearest end so they never vanish from the report.
func buildDeliveryRatePoints(rows []deliveryRow, from, to time.Time) []DeliveryRatePoint {
	keys := dayKeys(from, to)
	if len(keys) == 0 {
		return []DeliveryRatePoint{}
	}

	m := make(map[string]*DeliveryRatePoint, len(keys))
	for _, key := range keys {
		m[key] = &DeliveryRatePoint{Date: key}
	}

	first, last := keys[0], keys[len(keys)-1]
	for _, row := range rows {
		p, ok := m[row.Date]
		if !ok {
			p = m[clampKey(row.Date, first, last)]
		}
		switch row.Status {
		case string(models.EmailStatusSent):
			p.Sent += row.Count
		case string(models.EmailStatusFailed):
			p.Failed += row.Count
		}
	}

	result := make([]DeliveryRatePoint, 0, len(keys))
	for _, key := range keys {
		p := m[key]
		p.Total = p.Sent + p.Failed
		if p.Total > 0 {
			p.DeliveryRate = float64(p.Sent) / float64(p.Total) * 100
		}
		result = append(result, *p)
	}
	return result
}

// clampKey keys are zero-padded ISO dates, so they compare correctly as strings.
func clampKey(key, first, last string) string {
	if key < first {
		return first
	}
	return last
}

func (r *AnalyticsRepository) AdminBounceRateTrends(from, to time.Time) ([]BounceRatePoint, error) {
	var rows []bounceRow
	err := r.db.Table("bounces").
		Select("TO_CHAR(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD') as date, type, COUNT(*) as count").
		Where("created_at >= ? AND created_at <= ?", from, to).
		Group("date, type").Order("date ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return buildBounceRatePoints(rows, from, to), nil
}

func buildBounceRatePoints(rows []bounceRow, from, to time.Time) []BounceRatePoint {
	keys := dayKeys(from, to)
	if len(keys) == 0 {
		return []BounceRatePoint{}
	}

	m := make(map[string]*BounceRatePoint, len(keys))
	for _, key := range keys {
		m[key] = &BounceRatePoint{Date: key}
	}

	first, last := keys[0], keys[len(keys)-1]
	for _, row := range rows {
		p, ok := m[row.Date]
		if !ok {
			p = m[clampKey(row.Date, first, last)]
		}
		switch row.Type {
		case "hard":
			p.Hard += row.Count
		case "soft":
			p.Soft += row.Count
		case "complaint":
			p.Complaint += row.Count
		}
	}

	result := make([]BounceRatePoint, 0, len(keys))
	for _, key := range keys {
		p := m[key]
		p.Total = p.Hard + p.Soft + p.Complaint
		result = append(result, *p)
	}
	return result
}

func (r *AnalyticsRepository) AdminLatencyPercentiles(from, to time.Time) (*LatencyPercentiles, error) {
	var result LatencyPercentiles
	err := r.db.Table("emails").
		Select(`
			COALESCE(PERCENTILE_CONT(0.50) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (sent_at - created_at))), 0) as p50,
			COALESCE(PERCENTILE_CONT(0.75) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (sent_at - created_at))), 0) as p75,
			COALESCE(PERCENTILE_CONT(0.90) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (sent_at - created_at))), 0) as p90,
			COALESCE(PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (sent_at - created_at))), 0) as p99,
			COALESCE(AVG(EXTRACT(EPOCH FROM (sent_at - created_at))), 0) as avg
		`).
		Where("status = 'sent' AND sent_at IS NOT NULL AND created_at >= ? AND created_at <= ?", from, to).
		Scan(&result).Error
	return &result, err
}

func (r *AnalyticsRepository) WorkspaceDailyCounts(workspaceID uint, from, to time.Time, status string) ([]DailyCount, error) {
	var results []DailyCount
	query := r.db.Table("emails").
		Select("TO_CHAR(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD') as date, COUNT(*) as count").
		Where("workspace_id = ? AND created_at >= ? AND created_at <= ?", workspaceID, from, to)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	err := query.Group("date").Order("date ASC").Find(&results).Error
	return results, err
}

func (r *AnalyticsRepository) WorkspaceStatusBreakdown(workspaceID uint, from, to time.Time) ([]StatusBreakdown, error) {
	var results []StatusBreakdown
	err := r.db.Table("emails").
		Select("status, COUNT(*) as count").
		Where("workspace_id = ? AND created_at >= ? AND created_at <= ?", workspaceID, from, to).
		Group("status").
		Find(&results).Error
	return results, err
}

func (r *AnalyticsRepository) WorkspaceDeliveryRateTrends(workspaceID uint, from, to time.Time) ([]DeliveryRatePoint, error) {
	var rows []deliveryRow
	err := r.db.Table("emails").
		Select("TO_CHAR(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD') as date, status, COUNT(*) as count").
		Where("workspace_id = ? AND created_at >= ? AND created_at <= ? AND status IN ?", workspaceID, from, to, []string{string(models.EmailStatusSent), string(models.EmailStatusFailed)}).
		Group("date, status").Order("date ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return buildDeliveryRatePoints(rows, from, to), nil
}

func (r *AnalyticsRepository) WorkspaceBounceRateTrends(workspaceID uint, from, to time.Time) ([]BounceRatePoint, error) {
	var rows []bounceRow
	err := r.db.Table("bounces").
		Select("TO_CHAR(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD') as date, type, COUNT(*) as count").
		Where("workspace_id = ? AND created_at >= ? AND created_at <= ?", workspaceID, from, to).
		Group("date, type").Order("date ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return buildBounceRatePoints(rows, from, to), nil
}

func (r *AnalyticsRepository) WorkspaceLatencyPercentiles(workspaceID uint, from, to time.Time) (*LatencyPercentiles, error) {
	var result LatencyPercentiles
	err := r.db.Table("emails").
		Select(`
			COALESCE(PERCENTILE_CONT(0.50) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (sent_at - created_at))), 0) as p50,
			COALESCE(PERCENTILE_CONT(0.75) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (sent_at - created_at))), 0) as p75,
			COALESCE(PERCENTILE_CONT(0.90) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (sent_at - created_at))), 0) as p90,
			COALESCE(PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (sent_at - created_at))), 0) as p99,
			COALESCE(AVG(EXTRACT(EPOCH FROM (sent_at - created_at))), 0) as avg
		`).
		Where("workspace_id = ? AND status = 'sent' AND sent_at IS NOT NULL AND created_at >= ? AND created_at <= ?", workspaceID, from, to).
		Scan(&result).Error
	return &result, err
}

type ProviderBreakdownPoint struct {
	Provider string `json:"provider"`
	Sent     int64  `json:"sent"`
	Failed   int64  `json:"failed"`
	// Suppressed sends never reached the provider, so they say nothing about how it treats your mail.
	Suppressed   int64   `json:"suppressed"`
	Total        int64   `json:"total"`
	DeliveryRate float64 `json:"delivery_rate"`
}

type providerRow struct {
	Provider string
	Status   string
	Count    int64
}

// The provider column is set at send time by email.ClassifyRecipients, so this is a single indexed aggregate.
func (r *AnalyticsRepository) queryProviderRows(where string, args []any) ([]providerRow, error) {
	var rows []providerRow
	err := r.db.Table("emails").
		Select("COALESCE(NULLIF(provider, ''), 'Other') as provider, status, COUNT(*) as count").
		Where(where, args...).
		Group("provider, status").
		Scan(&rows).Error
	return rows, err
}

func (r *AnalyticsRepository) WorkspaceProviderBreakdown(workspaceID uint, from, to time.Time) ([]ProviderBreakdownPoint, error) {
	rows, err := r.queryProviderRows(
		"workspace_id = ? AND created_at >= ? AND created_at <= ?",
		[]any{workspaceID, from, to},
	)
	if err != nil {
		return nil, err
	}
	return buildProviderBreakdown(rows), nil
}

func (r *AnalyticsRepository) AdminProviderBreakdown(from, to time.Time) ([]ProviderBreakdownPoint, error) {
	rows, err := r.queryProviderRows(
		"created_at >= ? AND created_at <= ?",
		[]any{from, to},
	)
	if err != nil {
		return nil, err
	}
	return buildProviderBreakdown(rows), nil
}

func buildProviderBreakdown(rows []providerRow) []ProviderBreakdownPoint {
	agg := make(map[string]*ProviderBreakdownPoint)
	for _, row := range rows {
		p, ok := agg[row.Provider]
		if !ok {
			p = &ProviderBreakdownPoint{Provider: row.Provider}
			agg[row.Provider] = p
		}
		switch row.Status {
		case "sent":
			p.Sent += row.Count
		case "failed":
			p.Failed += row.Count
		case "suppressed":
			p.Suppressed += row.Count
		}
		p.Total += row.Count
	}
	out := make([]ProviderBreakdownPoint, 0, len(agg))
	for _, p := range agg {
		deliverable := p.Sent + p.Failed
		if deliverable > 0 {
			p.DeliveryRate = float64(p.Sent) / float64(deliverable) * 100
		}
		out = append(out, *p)
	}
	// Tie-break by name: map iteration is random, and a table that reshuffles on every refresh is hard to read.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Total != out[j].Total {
			return out[i].Total > out[j].Total
		}
		return out[i].Provider < out[j].Provider
	})
	return out
}
