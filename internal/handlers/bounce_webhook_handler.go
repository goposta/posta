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

package handlers

import (
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/logger"
	"github.com/jkaninda/okapi"
)

type BounceWebhookHandler struct {
	subscriberRepo *repositories.SubscriberRepository
	emailRepo      *repositories.EmailRepository
	messageRepo    *repositories.CampaignMessageRepository
}

func NewBounceWebhookHandler(
	subscriberRepo *repositories.SubscriberRepository,
	emailRepo *repositories.EmailRepository,
	messageRepo *repositories.CampaignMessageRepository,
) *BounceWebhookHandler {
	return &BounceWebhookHandler{
		subscriberRepo: subscriberRepo,
		emailRepo:      emailRepo,
		messageRepo:    messageRepo,
	}
}

type BounceNotification struct {
	Body struct {
		Email     string `json:"email" required:"true" doc:"Bounced email address"`
		Type      string `json:"type" doc:"Bounce type: hard or soft" enum:"hard,soft"`
		EmailUUID string `json:"email_id" doc:"UUID of the original email"`
		Reason    string `json:"reason" doc:"Bounce reason"`
	} `json:"body"`
}

type BounceResponse struct {
	Processed bool   `json:"processed"`
	Action    string `json:"action"`
}

func (h *BounceWebhookHandler) HandleBounce(c *okapi.Context, req *BounceNotification) error {
	email := strings.ToLower(strings.TrimSpace(req.Body.Email))
	if email == "" {
		return c.AbortBadRequest("email is required")
	}

	bounceType := req.Body.Type
	if bounceType == "" {
		bounceType = "hard"
	}

	action := "recorded"

	if bounceType == "hard" {
		// Find all subscribers with this email (across all scopes)
		var subscribers []models.Subscriber
		if err := h.subscriberRepo.FindAllByEmail(email, &subscribers); err == nil {
			now := time.Now()
			for i := range subscribers {
				if subscribers[i].Status != models.SubscriberStatusBounced {
					subscribers[i].Status = models.SubscriberStatusBounced
					subscribers[i].UpdatedAt = &now
					_ = h.subscriberRepo.Update(&subscribers[i])
				}
			}
			action = "subscriber_bounced"
		}
	}

	if req.Body.EmailUUID != "" {
		em, err := h.emailRepo.FindByUUID(req.Body.EmailUUID)
		if err == nil {
			msg, err := h.messageRepo.FindByEmailID(em.ID)
			if err == nil {
				_ = h.messageRepo.UpdateBouncedAt(msg.ID)
				action = "message_bounced"
			}
		}
	}

	logger.Info("bounce processed", "email", email, "type", bounceType, "action", action)
	return ok(c, BounceResponse{Processed: true, Action: action})
}
