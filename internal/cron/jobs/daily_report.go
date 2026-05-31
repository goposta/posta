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

	"github.com/goposta/posta/internal/cron"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/hibiken/asynq"
	"github.com/jkaninda/logger"
)

const TypeDailyReport = "cron:daily-report"

// DailyReportPayload is the Asynq task payload for a per-workspace daily report.
type DailyReportPayload struct {
	WorkspaceID uint `json:"workspace_id"`
}

// DailyReportJob enqueues a report task for each workspace. The processed report
// is delivered to the workspace's owners and admins.
type DailyReportJob struct {
	workspaceRepo *repositories.WorkspaceRepository
}

// dailyReportTask implements cron.Job for enqueueing a single workspace's report.
type dailyReportTask struct {
	workspaceID uint
}

func (t *dailyReportTask) Type() string { return TypeDailyReport }
func (t *dailyReportTask) Payload() any { return DailyReportPayload{WorkspaceID: t.workspaceID} }

func NewDailyReportJob(workspaceRepo *repositories.WorkspaceRepository) *DailyReportJob {
	return &DailyReportJob{workspaceRepo: workspaceRepo}
}

func (j *DailyReportJob) Name() string     { return "daily-report" }
func (j *DailyReportJob) Schedule() string { return "0 7 * * *" } // daily at 07:00 UTC

func (j *DailyReportJob) Run(_ context.Context, client *asynq.Client) error {
	workspaces, err := j.workspaceRepo.FindAll()
	if err != nil {
		logger.Error("daily report: failed to find workspaces", "error", err)
		return err
	}

	enqueued := 0
	for _, ws := range workspaces {
		if err := cron.EnqueueJob(client, &dailyReportTask{workspaceID: ws.ID}, asynq.Queue("low")); err != nil {
			logger.Error("daily report: failed to enqueue", "workspace_id", ws.ID, "error", err)
			continue
		}
		enqueued++
	}

	logger.Info("daily report: enqueued tasks", "count", enqueued)
	return nil
}

// NewDailyReportTask creates an Asynq task for processing a daily report.
func NewDailyReportTask(workspaceID uint, opts ...asynq.Option) (*asynq.Task, error) {
	payload, err := json.Marshal(DailyReportPayload{WorkspaceID: workspaceID})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeDailyReport, payload, opts...), nil
}
