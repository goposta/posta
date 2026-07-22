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
