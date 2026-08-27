// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package cron

import (
	"encoding/json"

	"github.com/hibiken/asynq"
)

// Job represents a unit of work that can be enqueued as an Asynq task.
type Job interface {
	Type() string
	Payload() any
}

// EnqueueJob converts a Job into an Asynq task and enqueues it.
func EnqueueJob(client *asynq.Client, job Job, opts ...asynq.Option) error {
	payload, err := json.Marshal(job.Payload())
	if err != nil {
		return err
	}
	task := asynq.NewTask(job.Type(), payload, opts...)
	_, err = client.Enqueue(task)
	return err
}
