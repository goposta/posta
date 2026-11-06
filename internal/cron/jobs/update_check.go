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

// The minute is deliberately not :00 so every Posta install does not stampede the GitHub API at once.
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
