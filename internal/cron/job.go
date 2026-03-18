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
