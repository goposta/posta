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

package worker

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
)

type Producer struct {
	client     *asynq.Client
	maxRetries int
}

func NewProducer(redisOpt asynq.RedisClientOpt, maxRetries int) *Producer {
	client := asynq.NewClient(redisOpt)
	return &Producer{
		client:     client,
		maxRetries: maxRetries,
	}
}

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

// The TaskID derives from the campaign ID so double-clicks and retries collapse instead of fanning out twice.
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

// A per-run random suffix keeps successive batches distinct while the first kick-off from Send can still dedupe.
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

// Parsing is separate from inbound:process so a permanent parse failure never blocks the SMTP 250 OK.
func (p *Producer) EnqueueInboundParse(inboundEmailID uint) error {
	task, err := NewInboundParseTask(inboundEmailID,
		asynq.Queue(QueueTransactional),
		asynq.MaxRetry(p.maxRetries),
	)
	if err != nil {
		return fmt.Errorf("failed to create inbound parse task: %w", err)
	}
	if _, err := p.client.Enqueue(task); err != nil {
		return fmt.Errorf("failed to enqueue inbound parse task: %w", err)
	}
	return nil
}

func (p *Producer) EnqueueMessageProcess(messageID uint) error {
	task, err := NewMessageProcessTask(messageID,
		asynq.Queue(QueueTransactional),
		asynq.MaxRetry(p.maxRetries),
		asynq.TaskID(fmt.Sprintf("message:process:%d", messageID)),
	)
	if err != nil {
		return fmt.Errorf("failed to create message task: %w", err)
	}
	if _, err := p.client.Enqueue(task); err != nil {
		if errors.Is(err, asynq.ErrTaskIDConflict) {
			return nil
		}
		return fmt.Errorf("failed to enqueue message task: %w", err)
	}
	return nil
}

// The task ID makes a second enqueue of the same reply a no-op.
func (p *Producer) EnqueueMessageReply(replyID uint) error {
	task, err := NewMessageReplyTask(replyID,
		asynq.Queue(QueueTransactional),
		asynq.MaxRetry(p.maxRetries),
		asynq.TaskID(fmt.Sprintf("message:reply:%d", replyID)),
	)
	if err != nil {
		return fmt.Errorf("failed to create message reply task: %w", err)
	}
	if _, err := p.client.Enqueue(task); err != nil {
		if errors.Is(err, asynq.ErrTaskIDConflict) {
			return nil
		}
		return fmt.Errorf("failed to enqueue message reply task: %w", err)
	}
	return nil
}

func (p *Producer) Close() error {
	return p.client.Close()
}
