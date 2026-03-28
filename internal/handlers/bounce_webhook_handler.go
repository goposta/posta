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

package handlers

import (
	"strings"
	"time"

	"github.com/jkaninda/logger"
	"github.com/jkaninda/okapi"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/storage/repositories"
)

// BounceWebhookHandler processes inbound bounce notifications.
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

	// Mark subscriber as bounced for hard bounces
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

	// Update campaign message if email UUID is provided
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
