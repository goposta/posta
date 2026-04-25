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
	"errors"
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

// EnqueueCampaignStart enqueues a campaign start task (fan-out to subscribers).
// The TaskID is derived from the campaign ID so double-clicks and retries
// collapse into a single Redis entry instead of fanning out twice.
func (p *Producer) EnqueueCampaignStart(campaignID uint) error {
	task, err := NewCampaignStartTask(
		campaignID,
		asynq.Queue(QueueBulk),
		asynq.MaxRetry(3),
		asynq.TaskID(fmt.Sprintf("campaign:start:%d", campaignID)),
	)
	if err != nil {
		return fmt.Errorf("failed to create campaign start task: %w", err)
	}
	if _, err := p.client.Enqueue(task); err != nil {
		// asynq.ErrTaskIDConflict means the task is already pending: treat as success.
		if errors.Is(err, asynq.ErrTaskIDConflict) {
			return nil
		}
		return fmt.Errorf("failed to enqueue campaign start: %w", err)
	}
	return nil
}

// EnqueueCampaignBatch enqueues a campaign batch processing task. A per-run
// random suffix keeps successive batches distinct while still letting us dedupe
// the *first* kick-off from Send (callers that want dedupe pass delay=0 after
// transitioning status atomically).
func (p *Producer) EnqueueCampaignBatch(campaignID uint, delay time.Duration) error {
	opts := []asynq.Option{asynq.Queue(QueueBulk), asynq.MaxRetry(3)}
	if delay > 0 {
		opts = append(opts, asynq.ProcessIn(delay))
	}
	task, err := NewCampaignBatchTask(campaignID, opts...)
	if err != nil {
		return fmt.Errorf("failed to create campaign batch task: %w", err)
	}
	_, err = p.client.Enqueue(task)
	return err
}

// EnqueueInboundProcess enqueues an inbound-process task that dispatches the
// email.inbound webhook for a received message.
func (p *Producer) EnqueueInboundProcess(inboundEmailID uint) error {
	task, err := NewInboundProcessTask(inboundEmailID,
		asynq.Queue(QueueTransactional),
		asynq.MaxRetry(p.maxRetries),
	)
	if err != nil {
		return fmt.Errorf("failed to create inbound task: %w", err)
	}
	if _, err := p.client.Enqueue(task); err != nil {
		return fmt.Errorf("failed to enqueue inbound task: %w", err)
	}
	return nil
}

// Close closes the underlying Asynq client.
func (p *Producer) Close() error {
	return p.client.Close()
}
