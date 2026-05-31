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

// BounceAlertJob checks for workspaces with high bounce rates and alerts their
// owners and admins.
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
func (j *BounceAlertJob) Schedule() string { return "0 9 * * *" } // daily at 09:00 UTC

func (j *BounceAlertJob) Run(_ context.Context, _ *asynq.Client) error {
	if j.notifier == nil || !j.notifier.IsConfigured() {
		return nil
	}

	now := time.Now().UTC()
	from := now.Add(-24 * time.Hour)

	// Find workspaces that sent emails in the last 24 hours
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

		// Count new suppressions
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
