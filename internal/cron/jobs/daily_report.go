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
	"encoding/json"

	"github.com/goposta/posta/internal/cron"
	"github.com/goposta/posta/internal/queues"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/hibiken/asynq"
	"github.com/jkaninda/logger"
)

const TypeDailyReport = "cron:daily-report"

type DailyReportPayload struct {
	WorkspaceID uint `json:"workspace_id"`
}

// DailyReportJob enqueues one report task per workspace, delivered to its owners and admins.
type DailyReportJob struct {
	workspaceRepo *repositories.WorkspaceRepository
}

type dailyReportTask struct {
	workspaceID uint
}

func (t *dailyReportTask) Type() string { return TypeDailyReport }
func (t *dailyReportTask) Payload() any { return DailyReportPayload{WorkspaceID: t.workspaceID} }

func NewDailyReportJob(workspaceRepo *repositories.WorkspaceRepository) *DailyReportJob {
	return &DailyReportJob{workspaceRepo: workspaceRepo}
}

func (j *DailyReportJob) Name() string     { return "daily-report" }
func (j *DailyReportJob) Schedule() string { return "0 7 * * *" }

func (j *DailyReportJob) Run(_ context.Context, client *asynq.Client) error {
	workspaces, err := j.workspaceRepo.FindAll()
	if err != nil {
		logger.Error("daily report: failed to find workspaces", "error", err)
		return err
	}

	enqueued := 0
	for _, ws := range workspaces {
		if err := cron.EnqueueJob(client, &dailyReportTask{workspaceID: ws.ID}, asynq.Queue(queues.Low)); err != nil {
			logger.Error("daily report: failed to enqueue", "workspace_id", ws.ID, "error", err)
			continue
		}
		enqueued++
	}

	logger.Info("daily report: enqueued tasks", "count", enqueued)
	return nil
}

func NewDailyReportTask(workspaceID uint, opts ...asynq.Option) (*asynq.Task, error) {
	payload, err := json.Marshal(DailyReportPayload{WorkspaceID: workspaceID})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeDailyReport, payload, opts...), nil
}
