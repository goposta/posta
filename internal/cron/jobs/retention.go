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
	"time"

	"github.com/hibiken/asynq"
	"github.com/jkaninda/logger"
	"github.com/jkaninda/posta/internal/services/settings"
	"github.com/jkaninda/posta/internal/storage/repositories"
)

// RetentionCleanupJob purges old email logs, audit events, and webhook
// delivery records based on platform retention settings.
type RetentionCleanupJob struct {
	emailRepo      *repositories.EmailRepository
	eventRepo      *repositories.EventRepository
	whDeliveryRepo *repositories.WebhookDeliveryRepository
	settings       *settings.Provider
}

func NewRetentionCleanupJob(
	emailRepo *repositories.EmailRepository,
	eventRepo *repositories.EventRepository,
	whDeliveryRepo *repositories.WebhookDeliveryRepository,
	sp *settings.Provider,
) *RetentionCleanupJob {
	return &RetentionCleanupJob{
		emailRepo:      emailRepo,
		eventRepo:      eventRepo,
		whDeliveryRepo: whDeliveryRepo,
		settings:       sp,
	}
}

func (j *RetentionCleanupJob) Name() string     { return "retention-cleanup" }
func (j *RetentionCleanupJob) Schedule() string { return "0 3 * * *" } // daily at 03:00 UTC

func (j *RetentionCleanupJob) Run(_ context.Context, _ *asynq.Client) error {
	// Clean up email logs
	emailRetention := j.settings.RetentionDays()
	if emailRetention > 0 {
		before := time.Now().AddDate(0, 0, -emailRetention)
		deleted, err := j.emailRepo.DeleteOlderThan(before)
		if err != nil {
			logger.Error("retention cleanup: failed to delete old emails", "error", err)
		} else if deleted > 0 {
			logger.Info("retention cleanup: deleted old emails", "count", deleted, "older_than_days", emailRetention)
		}
	}

	// Clean up audit/event logs
	auditRetention := j.settings.AuditLogRetentionDays()
	if auditRetention > 0 {
		before := time.Now().AddDate(0, 0, -auditRetention)
		deleted, err := j.eventRepo.DeleteOlderThan(before)
		if err != nil {
			logger.Error("retention cleanup: failed to delete old events", "error", err)
		} else if deleted > 0 {
			logger.Info("retention cleanup: deleted old events", "count", deleted, "older_than_days", auditRetention)
		}
	}

	// Clean up webhook delivery logs
	whRetention := j.settings.WebhookDeliveryRetentionDays()
	if whRetention > 0 {
		before := time.Now().AddDate(0, 0, -whRetention)
		deleted, err := j.whDeliveryRepo.DeleteOlderThan(before)
		if err != nil {
			logger.Error("retention cleanup: failed to delete old webhook deliveries", "error", err)
		} else if deleted > 0 {
			logger.Info("retention cleanup: deleted old webhook deliveries", "count", deleted, "older_than_days", whRetention)
		}
	}

	return nil
}
