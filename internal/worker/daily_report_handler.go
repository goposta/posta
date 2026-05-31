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

package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/goposta/posta/internal/cron/jobs"
	"github.com/goposta/posta/internal/services/notification"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/hibiken/asynq"
	"github.com/jkaninda/logger"
)

// DailyReportHandler processes daily report tasks from the Asynq queue.
type DailyReportHandler struct {
	notifier      *notification.Service
	analyticsRepo *repositories.AnalyticsRepository
	bounceRepo    *repositories.BounceRepository
}

// NewDailyReportHandler creates a new daily report task handler.
func NewDailyReportHandler(
	notifier *notification.Service,
	analyticsRepo *repositories.AnalyticsRepository,
	bounceRepo *repositories.BounceRepository,
) *DailyReportHandler {
	return &DailyReportHandler{
		notifier:      notifier,
		analyticsRepo: analyticsRepo,
		bounceRepo:    bounceRepo,
	}
}

// ProcessTask handles a daily report task for a single workspace.
func (h *DailyReportHandler) ProcessTask(_ context.Context, t *asynq.Task) error {
	var payload jobs.DailyReportPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("daily report: invalid payload: %w", err)
	}

	workspaceID := payload.WorkspaceID
	logger.Info("daily report: processing", "workspace_id", workspaceID)

	// Yesterday's date range
	now := time.Now().UTC()
	yesterday := now.AddDate(0, 0, -1)
	from := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, time.UTC)
	to := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 23, 59, 59, 0, time.UTC)

	// Gather stats
	breakdown, err := h.analyticsRepo.WorkspaceStatusBreakdown(workspaceID, from, to)
	if err != nil {
		logger.Error("daily report: failed to get stats", "workspace_id", workspaceID, "error", err)
		return err
	}

	var sent, failed, suppressed, total int64
	for _, s := range breakdown {
		total += s.Count
		switch s.Status {
		case "sent":
			sent = s.Count
		case "failed":
			failed = s.Count
		case "suppressed":
			suppressed = s.Count
		}
	}

	// Count bounces for the period
	var bounced int64
	if h.bounceRepo != nil {
		bounced, _ = h.bounceRepo.CountByWorkspaceAndDateRange(workspaceID, from, to)
	}

	var deliveryRate float64
	if total > 0 {
		deliveryRate = float64(sent) / float64(total) * 100
	}

	data := map[string]any{
		"ReportDate":       yesterday.Format("January 2, 2006"),
		"TotalEmails":      total,
		"SentEmails":       sent,
		"FailedEmails":     failed,
		"BouncedEmails":    bounced,
		"SuppressedEmails": suppressed,
		"DeliveryRate":     fmt.Sprintf("%.1f", deliveryRate),
	}

	subject := fmt.Sprintf("Daily Email Report — %s", yesterday.Format("Jan 2, 2006"))
	if err := h.notifier.SendToWorkspaceAdmins(workspaceID, subject, notification.TemplateDailyReport, data); err != nil {
		logger.Error("daily report: failed to send", "workspace_id", workspaceID, "error", err)
		return err
	}

	logger.Info("daily report: sent", "workspace_id", workspaceID)
	return nil
}
