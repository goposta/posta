// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package cron

import (
	"context"

	"github.com/hibiken/asynq"
)

// CronJob defines a recurring job that enqueues work on a cron schedule.
type CronJob interface {
	Name() string
	// cron expression
	Schedule() string
	Run(ctx context.Context, client *asynq.Client) error
}
