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

package handlers

import (
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/cache"
	"github.com/goposta/posta/internal/services/inbox"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/logger"
	"github.com/jkaninda/okapi"
	"gorm.io/gorm"
)

type DashboardHandler struct {
	db             *gorm.DB
	cache          *cache.Cache
	whDeliveryRepo *repositories.WebhookDeliveryRepository
	features       DashboardFeatures
	inbox          *inbox.Service
}

type DashboardFeatures struct {
	Messages bool `json:"messages"`
	Inbound  bool `json:"inbound"`
	Relay    bool `json:"relay"`
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
	TotalInbound      int64         `json:"total_inbound"`
	ForwardedInbound  int64         `json:"forwarded_inbound"`
	FailedInbound     int64         `json:"failed_inbound"`
	DailyVolume       []DailyVolume `json:"daily_volume"`

	UnverifiedDomains int64   `json:"unverified_domains"`
	ExpiringAPIKeys   int64   `json:"expiring_api_keys"`
	BounceRate        float64 `json:"bounce_rate"`

	TotalForms      int64 `json:"total_forms"`
	TotalMessages   int64 `json:"total_messages"`
	UnreadMessages  int64 `json:"unread_messages"`
	SpamMessages    int64 `json:"spam_messages"`
	TotalTemplates  int64 `json:"total_templates"`
	TotalCampaigns  int64 `json:"total_campaigns"`
	TotalSubscriber int64 `json:"total_subscribers"`

	Features DashboardFeatures `json:"features"`

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

func (h *DashboardHandler) SetFeatures(f DashboardFeatures) { h.features = f }

// SetInbox is optional: without it the dashboard still works but stops maintaining the health banner.
func (h *DashboardHandler) SetInbox(s *inbox.Service) { h.inbox = s }

// syncInbox runs only on fresh stats so the banner matches the numbers; its failures are not the user's problem.
func (h *DashboardHandler) syncInbox(c *okapi.Context, scope repositories.ResourceScope, stats *DashboardStats) {
	if h.inbox == nil || scope.WorkspaceID == nil {
		return
	}
	role := workspaceRole(c)
	if role == "" {
		return
	}
	snap := inbox.Snapshot{
		UnverifiedDomains: stats.UnverifiedDomains,
		ExpiringAPIKeys:   stats.ExpiringAPIKeys,
		UnreadMessages:    stats.UnreadMessages,
		TotalEmails:       stats.TotalEmails,
		BounceRate:        stats.BounceRate,
		FailureRate:       stats.FailureRate,
		MessagesEnabled:   h.features.Messages,
	}
	if err := h.inbox.SyncWorkspaceHealth(scope.UserID, *scope.WorkspaceID, role, snap); err != nil {
		logger.Warn("dashboard: failed to sync inbox alerts", "error", err, "workspace_id", *scope.WorkspaceID)
	}
}

func (h *DashboardHandler) Stats(c *okapi.Context) error {
	scope := getScope(c)
	ctx := c.Request().Context()

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

	applyScope(&models.Email{}).Count(&stats.TotalEmails)
	applyScope(&models.Email{}).Where("status = ?", models.EmailStatusQueued).Count(&stats.QueuedEmails)
	applyScope(&models.Email{}).Where("status = ?", models.EmailStatusProcessing).Count(&stats.ProcessingEmails)
	applyScope(&models.Email{}).Where("status = ?", models.EmailStatusSent).Count(&stats.SentEmails)
	applyScope(&models.Email{}).Where("status = ?", models.EmailStatusFailed).Count(&stats.FailedEmails)
	applyScope(&models.Email{}).Where("status = ?", models.EmailStatusSuppressed).Count(&stats.SuppressedEmails)

	applyScope(&models.Domain{}).Count(&stats.TotalDomains)
	applyScope(&models.Domain{}).Where("ownership_verified = false").Count(&stats.UnverifiedDomains)
	applyScope(&models.SMTPServer{}).Count(&stats.TotalSmtpServers)

	now := time.Now()
	applyScope(&models.APIKey{}).Count(&stats.TotalAPIKeys)
	applyScope(&models.APIKey{}).Where("revoked = false AND (expires_at IS NULL OR expires_at > ?)", now).Count(&stats.ActiveAPIKeys)
	applyScope(&models.APIKey{}).
		Where("revoked = false AND expires_at IS NOT NULL AND expires_at > ? AND expires_at <= ?", now, now.AddDate(0, 0, 7)).
		Count(&stats.ExpiringAPIKeys)

	applyScope(&models.Contact{}).Count(&stats.TotalContacts)
	applyScope(&models.Bounce{}).Count(&stats.TotalBounces)
	applyScope(&models.Suppression{}).Count(&stats.TotalSuppressions)
	applyScope(&models.Webhook{}).Count(&stats.TotalWebhooks)

	applyScope(&models.InboundEmail{}).Count(&stats.TotalInbound)
	applyScope(&models.InboundEmail{}).Where("status = ?", models.InboundStatusForwarded).Count(&stats.ForwardedInbound)
	applyScope(&models.InboundEmail{}).Where("status = ?", models.InboundStatusFailed).Count(&stats.FailedInbound)

	applyScope(&models.Template{}).Count(&stats.TotalTemplates)
	applyScope(&models.Campaign{}).Count(&stats.TotalCampaigns)
	applyScope(&models.Subscriber{}).Count(&stats.TotalSubscriber)

	if h.features.Messages {
		applyScope(&models.Form{}).Count(&stats.TotalForms)
		applyScope(&models.Message{}).
			Where("status <> ? AND deleted_at IS NULL", models.MessageStatusRejected).
			Count(&stats.TotalMessages)
		applyScope(&models.Message{}).
			Where("read_at IS NULL AND deleted_at IS NULL AND status IN ?",
				[]models.MessageStatus{models.MessageStatusReceived, models.MessageStatusFlagged}).
			Count(&stats.UnreadMessages)
		applyScope(&models.Message{}).
			Where("deleted_at IS NULL AND status IN ?",
				[]models.MessageStatus{models.MessageStatusQuarantined, models.MessageStatusRejected}).
			Count(&stats.SpamMessages)
	}

	if stats.TotalEmails > 0 {
		stats.FailureRate = float64(stats.FailedEmails) / float64(stats.TotalEmails) * 100
		stats.BounceRate = float64(stats.TotalBounces) / float64(stats.TotalEmails) * 100
	}

	stats.Features = h.features

	if whStats, err := h.whDeliveryRepo.StatsByScope(scope); err == nil {
		stats.WebhookDeliveries = whStats
	}

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

	h.cache.Set(ctx, cacheKey, stats, cache.DashboardStatsTTL)

	h.syncInbox(c, scope, &stats)

	return ok(c, stats)
}
