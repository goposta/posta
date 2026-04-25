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

	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/hibiken/asynq"
	"github.com/jkaninda/logger"
)

type CampaignBatchEnqueuer interface {
	EnqueueCampaignBatch(campaignID uint, delay time.Duration) error
}

type CampaignRestartJob struct {
	campaignRepo *repositories.CampaignRepository
	messageRepo  *repositories.CampaignMessageRepository
	producer     CampaignBatchEnqueuer
}

func NewCampaignRestartJob(
	campaignRepo *repositories.CampaignRepository,
	messageRepo *repositories.CampaignMessageRepository,
	producer CampaignBatchEnqueuer,
) *CampaignRestartJob {
	return &CampaignRestartJob{
		campaignRepo: campaignRepo,
		messageRepo:  messageRepo,
		producer:     producer,
	}
}

func (j *CampaignRestartJob) Name() string     { return "campaign-restart" }
func (j *CampaignRestartJob) Schedule() string { return "*/5 * * * *" }

const stuckFor = 10 * time.Minute

func (j *CampaignRestartJob) Run(_ context.Context, _ *asynq.Client) error {
	if j.producer == nil {
		return nil
	}
	stuck, err := j.campaignRepo.FindStuckSending(stuckFor)
	if err != nil {
		return err
	}
	for _, c := range stuck {
		pending, err := j.messageRepo.CountPending(c.ID)
		if err != nil {
			logger.Warn("campaign-restart: failed to count pending", "campaign_id", c.ID, "error", err)
			continue
		}
		if pending == 0 {
			continue
		}
		if err := j.producer.EnqueueCampaignBatch(c.ID, 0); err != nil {
			logger.Warn("campaign-restart: failed to re-enqueue batch", "campaign_id", c.ID, "error", err)
			continue
		}
		logger.Info("campaign-restart: re-enqueued stuck campaign", "campaign_id", c.ID, "pending", pending)
	}
	return nil
}
