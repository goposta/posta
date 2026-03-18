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

package repositories

import (
	"time"

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

// DailyCounts returns email counts per day for a user within a date range, optionally filtered by status.
func (r *AnalyticsRepository) DailyCounts(userID uint, from, to time.Time, status string) ([]DailyCount, error) {
	var results []DailyCount
	query := r.db.Table("emails").
		Select("TO_CHAR(created_at, 'YYYY-MM-DD') as date, COUNT(*) as count").
		Where("user_id = ? AND created_at >= ? AND created_at <= ?", userID, from, to)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	err := query.Group("date").Order("date ASC").Find(&results).Error
	return results, err
}

// StatusBreakdown returns counts grouped by status for a user within a date range.
func (r *AnalyticsRepository) StatusBreakdown(userID uint, from, to time.Time) ([]StatusBreakdown, error) {
	var results []StatusBreakdown
	err := r.db.Table("emails").
		Select("status, COUNT(*) as count").
		Where("user_id = ? AND created_at >= ? AND created_at <= ?", userID, from, to).
		Group("status").
		Find(&results).Error
	return results, err
}

// HourlyCounts returns email counts per hour for a user on a specific day.
func (r *AnalyticsRepository) HourlyCounts(userID uint, date time.Time) ([]DailyCount, error) {
	var results []DailyCount
	start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	end := start.Add(24 * time.Hour)
	err := r.db.Table("emails").
		Select("TO_CHAR(created_at, 'HH24:00') as date, COUNT(*) as count").
		Where("user_id = ? AND created_at >= ? AND created_at < ?", userID, start, end).
		Group("date").Order("date ASC").Find(&results).Error
	return results, err
}

// AdminDailyCounts returns email counts across all users.
func (r *AnalyticsRepository) AdminDailyCounts(from, to time.Time, status string) ([]DailyCount, error) {
	var results []DailyCount
	query := r.db.Table("emails").
		Select("TO_CHAR(created_at, 'YYYY-MM-DD') as date, COUNT(*) as count").
		Where("created_at >= ? AND created_at <= ?", from, to)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	err := query.Group("date").Order("date ASC").Find(&results).Error
	return results, err
}

// AdminStatusBreakdown returns counts grouped by status across all users.
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

// DeliveryRatePoint represents a single day's delivery rate.
type DeliveryRatePoint struct {
	Date         string  `json:"date"`
	Sent         int64   `json:"sent"`
	Failed       int64   `json:"failed"`
	Total        int64   `json:"total"`
	DeliveryRate float64 `json:"delivery_rate"`
}

// BounceRatePoint represents a single day's bounce counts by type.
type BounceRatePoint struct {
	Date      string `json:"date"`
	Hard      int64  `json:"hard"`
	Soft      int64  `json:"soft"`
	Complaint int64  `json:"complaint"`
	Total     int64  `json:"total"`
}

// LatencyPercentiles represents email delivery latency percentiles.
type LatencyPercentiles struct {
	P50 float64 `json:"p50"`
	P75 float64 `json:"p75"`
	P90 float64 `json:"p90"`
	P99 float64 `json:"p99"`
	Avg float64 `json:"avg"`
}

// DeliveryRateTrends returns daily delivery rate for a user within a date range.
func (r *AnalyticsRepository) DeliveryRateTrends(userID uint, from, to time.Time) ([]DeliveryRatePoint, error) {
	var rows []deliveryRow
	err := r.db.Table("emails").
		Select("TO_CHAR(created_at, 'YYYY-MM-DD') as date, status, COUNT(*) as count").
		Where("user_id = ? AND created_at >= ? AND created_at <= ? AND status IN ?", userID, from, to, []string{"sent", "failed"}).
		Group("date, status").Order("date ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return buildDeliveryRatePoints(rows, from, to), nil
}

// AdminDeliveryRateTrends returns daily delivery rate across all users.
func (r *AnalyticsRepository) AdminDeliveryRateTrends(from, to time.Time) ([]DeliveryRatePoint, error) {
	var rows []deliveryRow
	err := r.db.Table("emails").
		Select("TO_CHAR(created_at, 'YYYY-MM-DD') as date, status, COUNT(*) as count").
		Where("created_at >= ? AND created_at <= ? AND status IN ?", from, to, []string{"sent", "failed"}).
		Group("date, status").Order("date ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return buildDeliveryRatePoints(rows, from, to), nil
}

func buildDeliveryRatePoints(rows []deliveryRow, from, to time.Time) []DeliveryRatePoint {
	m := make(map[string]*DeliveryRatePoint)
	start := from.Truncate(24 * time.Hour)
	end := to.Truncate(24 * time.Hour)
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		m[key] = &DeliveryRatePoint{Date: key}
	}
	for _, r := range rows {
		p, ok := m[r.Date]
		if !ok {
			p = &DeliveryRatePoint{Date: r.Date}
			m[r.Date] = p
		}
		switch r.Status {
		case "sent":
			p.Sent = r.Count
		case "failed":
			p.Failed = r.Count
		}
	}
	result := make([]DeliveryRatePoint, 0, len(m))
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		p := m[key]
		p.Total = p.Sent + p.Failed
		if p.Total > 0 {
			p.DeliveryRate = float64(p.Sent) / float64(p.Total) * 100
		}
		result = append(result, *p)
	}
	return result
}

// BounceRateTrends returns daily bounce counts by type for a user.
func (r *AnalyticsRepository) BounceRateTrends(userID uint, from, to time.Time) ([]BounceRatePoint, error) {
	var rows []bounceRow
	err := r.db.Table("bounces").
		Select("TO_CHAR(created_at, 'YYYY-MM-DD') as date, type, COUNT(*) as count").
		Where("user_id = ? AND created_at >= ? AND created_at <= ?", userID, from, to).
		Group("date, type").Order("date ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return buildBounceRatePoints(rows, from, to), nil
}

// AdminBounceRateTrends returns daily bounce counts by type across all users.
func (r *AnalyticsRepository) AdminBounceRateTrends(from, to time.Time) ([]BounceRatePoint, error) {
	var rows []bounceRow
	err := r.db.Table("bounces").
		Select("TO_CHAR(created_at, 'YYYY-MM-DD') as date, type, COUNT(*) as count").
		Where("created_at >= ? AND created_at <= ?", from, to).
		Group("date, type").Order("date ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return buildBounceRatePoints(rows, from, to), nil
}

func buildBounceRatePoints(rows []bounceRow, from, to time.Time) []BounceRatePoint {
	m := make(map[string]*BounceRatePoint)
	start := from.Truncate(24 * time.Hour)
	end := to.Truncate(24 * time.Hour)
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		m[key] = &BounceRatePoint{Date: key}
	}
	for _, r := range rows {
		p, ok := m[r.Date]
		if !ok {
			p = &BounceRatePoint{Date: r.Date}
			m[r.Date] = p
		}
		switch r.Type {
		case "hard":
			p.Hard = r.Count
		case "soft":
			p.Soft = r.Count
		case "complaint":
			p.Complaint = r.Count
		}
	}
	result := make([]BounceRatePoint, 0, len(m))
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		p := m[key]
		p.Total = p.Hard + p.Soft + p.Complaint
		result = append(result, *p)
	}
	return result
}

// LatencyPercentilesForUser returns delivery latency percentiles for a user.
func (r *AnalyticsRepository) LatencyPercentilesForUser(userID uint, from, to time.Time) (*LatencyPercentiles, error) {
	var result LatencyPercentiles
	err := r.db.Table("emails").
		Select(`
			COALESCE(PERCENTILE_CONT(0.50) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (sent_at - created_at))), 0) as p50,
			COALESCE(PERCENTILE_CONT(0.75) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (sent_at - created_at))), 0) as p75,
			COALESCE(PERCENTILE_CONT(0.90) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (sent_at - created_at))), 0) as p90,
			COALESCE(PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (sent_at - created_at))), 0) as p99,
			COALESCE(AVG(EXTRACT(EPOCH FROM (sent_at - created_at))), 0) as avg
		`).
		Where("user_id = ? AND status = 'sent' AND sent_at IS NOT NULL AND created_at >= ? AND created_at <= ?", userID, from, to).
		Scan(&result).Error
	return &result, err
}

// AdminLatencyPercentiles returns delivery latency percentiles across all users.
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
