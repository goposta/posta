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
	"encoding/json"

	"github.com/hibiken/asynq"
	"github.com/jkaninda/logger"
	"github.com/jkaninda/posta/internal/cron"
	"github.com/jkaninda/posta/internal/storage/repositories"
)

const TypeDailyReport = "cron:daily-report"

// DailyReportPayload is the Asynq task payload for a per-user daily report.
type DailyReportPayload struct {
	UserID uint `json:"user_id"`
}

// DailyReportJob is a daily report job that enqueues a report task for each user
// that has daily_report enabled in their settings.
type DailyReportJob struct {
	userSettingRepo *repositories.UserSettingRepository
}

// dailyReportTask implements cron.Job for enqueueing a single user's report.
type dailyReportTask struct {
	userID uint
}

func (t *dailyReportTask) Type() string { return TypeDailyReport }
func (t *dailyReportTask) Payload() any { return DailyReportPayload{UserID: t.userID} }

func NewDailyReportJob(userSettingRepo *repositories.UserSettingRepository) *DailyReportJob {
	return &DailyReportJob{userSettingRepo: userSettingRepo}
}

func (j *DailyReportJob) Name() string     { return "daily-report" }
func (j *DailyReportJob) Schedule() string { return "0 7 * * *" } // daily at 07:00 UTC

func (j *DailyReportJob) Run(_ context.Context, client *asynq.Client) error {
	users, err := j.userSettingRepo.FindUsersWithDailyReport()
	if err != nil {
		logger.Error("daily report: failed to find users", "error", err)
		return err
	}

	enqueued := 0
	for _, userID := range users {
		if err := cron.EnqueueJob(client, &dailyReportTask{userID: userID}, asynq.Queue("low")); err != nil {
			logger.Error("daily report: failed to enqueue", "user_id", userID, "error", err)
			continue
		}
		enqueued++
	}

	logger.Info("daily report: enqueued tasks", "count", enqueued)
	return nil
}

// NewDailyReportTask creates an Asynq task for processing a daily report.
func NewDailyReportTask(userID uint, opts ...asynq.Option) (*asynq.Task, error) {
	payload, err := json.Marshal(DailyReportPayload{UserID: userID})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeDailyReport, payload, opts...), nil
}
