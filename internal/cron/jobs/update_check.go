// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package jobs

import (
	"context"
	"time"

	"github.com/goposta/posta/internal/services/updatecheck"
	"github.com/hibiken/asynq"
	"github.com/jkaninda/logger"
)

// UpdateCheckJob asks GitHub once a day whether a newer Posta release exists.
type UpdateCheckJob struct {
	svc *updatecheck.Service
}

func NewUpdateCheckJob(svc *updatecheck.Service) *UpdateCheckJob {
	return &UpdateCheckJob{svc: svc}
}

func (j *UpdateCheckJob) Name() string { return "update-check" }

// Schedule runs daily. The minute is arbitrary but deliberately not :00 — every
// Posta in the world sharing one cron minute would stampede the GitHub API.
func (j *UpdateCheckJob) Schedule() string { return "37 4 * * *" }

func (j *UpdateCheckJob) Run(ctx context.Context, _ *asynq.Client) error {
	if j.svc == nil || !j.svc.Enabled() {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := j.svc.Check(ctx); err != nil {
		logger.Warn("update check failed", "error", err)
	}
	return nil
}
