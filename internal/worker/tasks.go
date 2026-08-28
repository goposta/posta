// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package worker

import (
	"encoding/json"

	"github.com/hibiken/asynq"
)

const (
	TypeEmailSend      = "email:send"
	TypeCampaignStart  = "campaign:start"
	TypeCampaignBatch  = "campaign:batch"
	TypeInboundParse   = "inbound:parse"
	TypeInboundProcess = "inbound:process"
	TypeMessageProcess = "message:process"

	QueueTransactional = "transactional"
	QueueBulk          = "bulk"
	QueueLow           = "low"
)

type EmailSendPayload struct {
	EmailID uint `json:"email_id"`
}

// NewEmailSendTask creates an Asynq task to send an email.
func NewEmailSendTask(emailID uint, opts ...asynq.Option) (*asynq.Task, error) {
	payload, err := json.Marshal(EmailSendPayload{EmailID: emailID})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeEmailSend, payload, opts...), nil
}

type CampaignPayload struct {
	CampaignID uint `json:"campaign_id"`
}

func NewCampaignStartTask(campaignID uint, opts ...asynq.Option) (*asynq.Task, error) {
	payload, err := json.Marshal(CampaignPayload{CampaignID: campaignID})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeCampaignStart, payload, opts...), nil
}

func NewCampaignBatchTask(campaignID uint, opts ...asynq.Option) (*asynq.Task, error) {
	payload, err := json.Marshal(CampaignPayload{CampaignID: campaignID})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeCampaignBatch, payload, opts...), nil
}

type InboundProcessPayload struct {
	InboundEmailID uint `json:"inbound_email_id"`
}

// NewInboundProcessTask creates an Asynq task to dispatch an inbound email's
// email.inbound webhook to subscribers.
func NewInboundProcessTask(id uint, opts ...asynq.Option) (*asynq.Task, error) {
	payload, err := json.Marshal(InboundProcessPayload{InboundEmailID: id})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeInboundProcess, payload, opts...), nil
}

type InboundParsePayload struct {
	InboundEmailID uint `json:"inbound_email_id"`
}

// NewInboundParseTask creates an Asynq task that parses a previously-received
// raw inbound message into its structured form.
func NewInboundParseTask(id uint, opts ...asynq.Option) (*asynq.Task, error) {
	payload, err := json.Marshal(InboundParsePayload{InboundEmailID: id})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeInboundParse, payload, opts...), nil
}

type MessageProcessPayload struct {
	MessageID uint `json:"message_id"`
}

func NewMessageProcessTask(id uint, opts ...asynq.Option) (*asynq.Task, error) {
	payload, err := json.Marshal(MessageProcessPayload{MessageID: id})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeMessageProcess, payload, opts...), nil
}
