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
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/cache"
	"github.com/goposta/posta/internal/storage/repositories"
	"gorm.io/gorm"
)

type DashboardHandler struct {
	db             *gorm.DB
	cache          *cache.Cache
	whDeliveryRepo *repositories.WebhookDeliveryRepository
}

type DashboardStats struct {
	TotalEmails       int64         `json:"total_emails"`
	QueuedEmails      int64         `json:"queued_emails"`
	ProcessingEmails  int64         `json:"processing_emails"`
	SentEmails        int64         `json:"sent_emails"`
	FailedEmails      int64         `json:"failed_emails"`
	SuppressedEmails  int64         `json:"suppressed_emails"`
	FailureRate       float64       `json:"failure_rate"`
	TotalDomains      int64         `json:"total_domains"`
	TotalSmtpServers  int64         `json:"total_smtp_servers"`
	TotalAPIKeys      int64         `json:"total_api_keys"`
	ActiveAPIKeys     int64         `json:"active_api_keys"`
	TotalContacts     int64         `json:"total_contacts"`
	TotalBounces      int64         `json:"total_bounces"`
	TotalSuppressions int64         `json:"total_suppressions"`
	TotalWebhooks     int64         `json:"total_webhooks"`
	DailyVolume       []DailyVolume `json:"daily_volume"`

	// Webhook delivery stats
	WebhookDeliveries *repositories.WebhookDeliveryStats `json:"webhook_deliveries"`
}

type DailyVolume struct {
	Date   string `json:"date"`
	Sent   int64  `json:"sent"`
	Failed int64  `json:"failed"`
}

func NewDashboardHandler(db *gorm.DB, c *cache.Cache, whDeliveryRepo *repositories.WebhookDeliveryRepository) *DashboardHandler {
	return &DashboardHandler{db: db, cache: c, whDeliveryRepo: whDeliveryRepo}
}

func (h *DashboardHandler) Stats(c *okapi.Context) error {
	scope := getScope(c)
	ctx := c.Request().Context()

	// Try cache first
	scopeKey := int(scope.UserID)
	if scope.WorkspaceID != nil {
		scopeKey = int(*scope.WorkspaceID) + 1000000
	}
	cacheKey := cache.DashboardKey(scopeKey)
	var stats DashboardStats
	if h.cache.Get(ctx, cacheKey, &stats) {
		return ok(c, stats)
	}

	applyScope := func(model interface{}) *gorm.DB {
		return repositories.ApplyScope(h.db.Model(model), scope)
	}

	// Email counts by status
	applyScope(&models.Email{}).Count(&stats.TotalEmails)
	applyScope(&models.Email{}).Where("status = ?", models.EmailStatusQueued).Count(&stats.QueuedEmails)
	applyScope(&models.Email{}).Where("status = ?", models.EmailStatusProcessing).Count(&stats.ProcessingEmails)
	applyScope(&models.Email{}).Where("status = ?", models.EmailStatusSent).Count(&stats.SentEmails)
	applyScope(&models.Email{}).Where("status = ?", models.EmailStatusFailed).Count(&stats.FailedEmails)
	applyScope(&models.Email{}).Where("status = ?", models.EmailStatusSuppressed).Count(&stats.SuppressedEmails)

	// Infrastructure counts
	applyScope(&models.Domain{}).Count(&stats.TotalDomains)
	applyScope(&models.SMTPServer{}).Count(&stats.TotalSmtpServers)

	// API keys
	applyScope(&models.APIKey{}).Count(&stats.TotalAPIKeys)
	applyScope(&models.APIKey{}).Where("revoked = false AND (expires_at IS NULL OR expires_at > ?)", time.Now()).Count(&stats.ActiveAPIKeys)

	// Contacts & deliverability
	applyScope(&models.Contact{}).Count(&stats.TotalContacts)
	applyScope(&models.Bounce{}).Count(&stats.TotalBounces)
	applyScope(&models.Suppression{}).Count(&stats.TotalSuppressions)
	applyScope(&models.Webhook{}).Count(&stats.TotalWebhooks)

	// Failure rate
	if stats.TotalEmails > 0 {
		stats.FailureRate = float64(stats.FailedEmails) / float64(stats.TotalEmails) * 100
	}

	// Webhook delivery stats
	if whStats, err := h.whDeliveryRepo.StatsByScope(scope); err == nil {
		stats.WebhookDeliveries = whStats
	}

	// Daily send volume (last 14 days)
	since := time.Now().AddDate(0, 0, -13).Truncate(24 * time.Hour)
	type dailyRow struct {
		Date   string
		Status string
		Count  int64
	}
	var rows []dailyRow
	applyScope(&models.Email{}).
		Select("TO_CHAR(created_at, 'YYYY-MM-DD') as date, status, COUNT(*) as count").
		Where("created_at >= ? AND status IN ?", since, []string{string(models.EmailStatusSent), string(models.EmailStatusFailed)}).
		Group("date, status").
		Order("date").
		Find(&rows)

	// Build a map for easy lookup and fill all 14 days
	volumeMap := make(map[string]*DailyVolume)
	for i := 0; i < 14; i++ {
		d := since.AddDate(0, 0, i).Format("2006-01-02")
		volumeMap[d] = &DailyVolume{Date: d}
	}
	for _, r := range rows {
		v, ok := volumeMap[r.Date]
		if !ok {
			continue
		}
		if r.Status == string(models.EmailStatusSent) {
			v.Sent = r.Count
		} else if r.Status == string(models.EmailStatusFailed) {
			v.Failed = r.Count
		}
	}
	stats.DailyVolume = make([]DailyVolume, 0, 14)
	for i := 0; i < 14; i++ {
		d := since.AddDate(0, 0, i).Format("2006-01-02")
		stats.DailyVolume = append(stats.DailyVolume, *volumeMap[d])
	}

	// Store in cache
	h.cache.Set(ctx, cacheKey, stats, cache.DashboardStatsTTL)

	return ok(c, stats)
}
