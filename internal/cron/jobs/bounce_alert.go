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

package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/notification"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/hibiken/asynq"
	"github.com/jkaninda/logger"
	"gorm.io/gorm"
)

const bounceRateThreshold = 5.0 // percent

// BounceAlertJob alerts a workspace's owners and admins when its bounce rate is high.
type BounceAlertJob struct {
	db              *gorm.DB
	notifier        *notification.Service
	bounceRepo      *repositories.BounceRepository
	suppressionRepo *repositories.SuppressionRepository
}

func NewBounceAlertJob(
	db *gorm.DB,
	notifier *notification.Service,
	bounceRepo *repositories.BounceRepository,
	suppressionRepo *repositories.SuppressionRepository,
) *BounceAlertJob {
	return &BounceAlertJob{
		db:              db,
		notifier:        notifier,
		bounceRepo:      bounceRepo,
		suppressionRepo: suppressionRepo,
	}
}

func (j *BounceAlertJob) Name() string     { return "bounce-alert" }
func (j *BounceAlertJob) Schedule() string { return "0 9 * * *" }

func (j *BounceAlertJob) Run(_ context.Context, _ *asynq.Client) error {
	if j.notifier == nil || !j.notifier.IsConfigured() {
		return nil
	}

	now := time.Now().UTC()
	from := now.Add(-24 * time.Hour)

	type workspaceEmailCount struct {
		WorkspaceID uint  `gorm:"column:workspace_id"`
		Total       int64 `gorm:"column:total"`
	}
	var counts []workspaceEmailCount
	if err := j.db.Model(&models.Email{}).
		Select("workspace_id, COUNT(*) as total").
		Where("created_at >= ? AND workspace_id IS NOT NULL", from).
		Group("workspace_id").
		Having("COUNT(*) >= ?", 10).
		Find(&counts).Error; err != nil {
		logger.Error("bounce-alert: failed to query email counts", "error", err)
		return err
	}

	sent := 0
	for _, wc := range counts {
		bounceCount, err := j.bounceRepo.CountByWorkspaceAndDateRange(wc.WorkspaceID, from, now)
		if err != nil {
			continue
		}

		bounceRate := float64(bounceCount) / float64(wc.Total) * 100
		if bounceRate < bounceRateThreshold {
			continue
		}

		var suppressionCount int64
		j.db.Model(&models.Suppression{}).
			Where("workspace_id = ? AND created_at >= ?", wc.WorkspaceID, from).
			Count(&suppressionCount)

		if err := j.notifier.SendToWorkspaceAdmins(wc.WorkspaceID, "Bounce Rate Alert", notification.TemplateBounceAlert, map[string]any{
			"BounceRate":       fmt.Sprintf("%.1f", bounceRate),
			"Threshold":        fmt.Sprintf("%.0f", bounceRateThreshold),
			"TotalEmails":      wc.Total,
			"BounceCount":      bounceCount,
			"SuppressionCount": suppressionCount,
		}); err != nil {
			logger.Error("bounce-alert: failed to send", "workspace_id", wc.WorkspaceID, "error", err)
			continue
		}
		sent++
	}

	logger.Info("bounce-alert: notifications sent", "count", sent)
	return nil
}
