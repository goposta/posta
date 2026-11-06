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

type DailyReportHandler struct {
	notifier      *notification.Service
	analyticsRepo *repositories.AnalyticsRepository
	bounceRepo    *repositories.BounceRepository
}

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

func (h *DailyReportHandler) ProcessTask(_ context.Context, t *asynq.Task) error {
	var payload jobs.DailyReportPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("daily report: invalid payload: %w", err)
	}

	workspaceID := payload.WorkspaceID
	logger.Info("daily report: processing", "workspace_id", workspaceID)

	now := time.Now().UTC()
	yesterday := now.AddDate(0, 0, -1)
	from := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, time.UTC)
	to := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 23, 59, 59, 0, time.UTC)

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
