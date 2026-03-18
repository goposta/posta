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

package worker

import (
	"fmt"
	"time"

	"github.com/hibiken/asynq"
)

// Producer enqueues email tasks into Redis via Asynq.
type Producer struct {
	client     *asynq.Client
	maxRetries int
}

func NewProducer(redisAddr, redisPassword string, maxRetries int) *Producer {
	client := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     redisAddr,
		Password: redisPassword,
	})
	return &Producer{
		client:     client,
		maxRetries: maxRetries,
	}
}

// EnqueueEmailSend enqueues an email for background delivery.
func (p *Producer) EnqueueEmailSend(emailID uint, queue string) error {
	if queue == "" {
		queue = QueueTransactional
	}
	task, err := NewEmailSendTask(emailID,
		asynq.Queue(queue),
		asynq.MaxRetry(p.maxRetries),
	)
	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}
	_, err = p.client.Enqueue(task)
	if err != nil {
		return fmt.Errorf("failed to enqueue task: %w", err)
	}
	return nil
}

// EnqueueEmailSendAt enqueues an email for delivery at the specified time.
func (p *Producer) EnqueueEmailSendAt(emailID uint, queue string, sendAt time.Time) error {
	if queue == "" {
		queue = QueueTransactional
	}
	delay := time.Until(sendAt)
	if delay < 0 {
		delay = 0
	}
	task, err := NewEmailSendTask(emailID,
		asynq.Queue(queue),
		asynq.MaxRetry(p.maxRetries),
		asynq.ProcessIn(delay),
	)
	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}
	_, err = p.client.Enqueue(task)
	if err != nil {
		return fmt.Errorf("failed to enqueue task: %w", err)
	}
	return nil
}

// Close closes the underlying Asynq client.
func (p *Producer) Close() error {
	return p.client.Close()
}
