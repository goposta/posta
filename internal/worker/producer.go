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

// Producer enqueues email tasks into Redis via Asynq.
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

// EnqueueInboundParse enqueues a parse task for a raw inbound message that
// was just persisted by the SMTP receiver. The parse step is intentionally
// separate from EnqueueInboundProcess so a permanent parse failure does not
// block the SMTP 250 OK reply.
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

// EnqueueMessageReply schedules the follow-up for a thread reply. The task ID
// makes a second enqueue of the same reply a no-op.
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

// EnqueueChatNotify schedules the notification for a new chat conversation.
// The task ID makes a second enqueue for the same conversation a no-op.
func (p *Producer) EnqueueChatNotify(conversationID uint) error {
	task, err := NewChatNotifyTask(conversationID,
		asynq.Queue(QueueTransactional),
		asynq.MaxRetry(p.maxRetries),
		asynq.TaskID(fmt.Sprintf("chat:notify:%d", conversationID)),
	)
	if err != nil {
		return fmt.Errorf("failed to create chat notify task: %w", err)
	}
	if _, err := p.client.Enqueue(task); err != nil {
		if errors.Is(err, asynq.ErrTaskIDConflict) {
			return nil
		}
		return fmt.Errorf("failed to enqueue chat notify task: %w", err)
	}
	return nil
}

// EnqueueChatOfflineEmail schedules the follow-up email for an agent's chat
// message, delay from now.
func (p *Producer) EnqueueChatOfflineEmail(conversationID uint, seq int64, delay time.Duration) error {
	return p.enqueueChatMessage(TypeChatOfflineEmail, conversationID, seq, delay)
}

// EnqueueChatUnanswered schedules the alert for a visitor's chat message,
// delay from now.
func (p *Producer) EnqueueChatUnanswered(conversationID uint, seq int64, delay time.Duration) error {
	return p.enqueueChatMessage(TypeChatUnanswered, conversationID, seq, delay)
}

// EnqueueChatWebhook queues a chat webhook event. Message events are
// enqueued once per message; conversation events each time they happen.
func (p *Producer) EnqueueChatWebhook(event string, conversationID, messageID uint) error {
	payload, err := json.Marshal(ChatWebhookPayload{Event: event, ConversationID: conversationID, MessageID: messageID})
	if err != nil {
		return err
	}
	opts := []asynq.Option{asynq.Queue(QueueLow), asynq.MaxRetry(p.maxRetries)}
	if messageID != 0 {
		opts = append(opts, asynq.TaskID(fmt.Sprintf("%s:%s:%d", TypeChatWebhook, event, messageID)))
	}
	if _, err := p.client.Enqueue(asynq.NewTask(TypeChatWebhook, payload, opts...)); err != nil {
		if errors.Is(err, asynq.ErrTaskIDConflict) {
			return nil
		}
		return fmt.Errorf("failed to enqueue chat webhook: %w", err)
	}
	return nil
}

// enqueueChatMessage enqueues one task per message; the task ID makes a
// second enqueue for the same message a no-op.
func (p *Producer) enqueueChatMessage(typ string, conversationID uint, seq int64, delay time.Duration) error {
	task, err := newChatMessageTask(typ, conversationID, seq,
		asynq.Queue(QueueTransactional),
		asynq.MaxRetry(p.maxRetries),
		asynq.ProcessIn(delay),
		asynq.TaskID(fmt.Sprintf("%s:%d:%d", typ, conversationID, seq)),
	)
	if err != nil {
		return fmt.Errorf("failed to create %s task: %w", typ, err)
	}
	if _, err := p.client.Enqueue(task); err != nil {
		if errors.Is(err, asynq.ErrTaskIDConflict) {
			return nil
		}
		return fmt.Errorf("failed to enqueue %s task: %w", typ, err)
	}
	return nil
}

// Close closes the underlying Asynq client.
func (p *Producer) Close() error {
	return p.client.Close()
}
