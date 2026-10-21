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
	"crypto/rand"
	"encoding/hex"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/audit"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
)

type WebhookHandler struct {
	repo  *repositories.WebhookRepository
	audit *audit.Logger
}
type CreateWebhookRequest struct {
	Body struct {
		URL string `json:"url" required:"true" format:"url"`
		// The enum must list dto.WebhookEvents in order; TestWebhookEnumMatchesRegistry enforces it.
		Events  []string `json:"events" required:"true" minItems:"1" enum:"email.sent,email.failed,email.inbound,email.unsubscribed,email.complained,message.received,message.spam,message.replied,message.reply_received,campaign.started,campaign.completed"`
		Filters []string `json:"filters"`
	} `json:"body"`
}
type DeleteWebhookRequest struct {
	ID int `param:"id"`
}

func NewWebhookHandler(repo *repositories.WebhookRepository, audit *audit.Logger) *WebhookHandler {
	return &WebhookHandler{repo: repo, audit: audit}
}

func (h *WebhookHandler) Create(c *okapi.Context, req *CreateWebhookRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("Insufficient workspace permissions", err)
	}
	scope := getScope(c)

	// Generate a random signing secret for HMAC webhook signatures
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return c.AbortInternalServerError("failed to generate webhook secret", err)
	}
	secret := hex.EncodeToString(secretBytes)

	wh := &models.Webhook{
		UserID:      scope.UserID,
		WorkspaceID: scope.WorkspaceID,
		URL:         req.Body.URL,
		Events:      req.Body.Events,
		Filters:     req.Body.Filters,
		Secret:      secret,
	}

	if err := h.repo.Create(wh); err != nil {
		return c.AbortInternalServerError("failed to create webhook", err)
	}

	h.audit.LogCtx(c, "webhook.created", "Webhook created: "+req.Body.URL, nil)

	return created(c, wh)
}

func (h *WebhookHandler) List(c *okapi.Context, req *ListRequest) error {
	page, size, offset := normalizePageParams(req.Page, req.Size)

	webhooks, total, err := h.repo.FindByScope(getScope(c), size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list webhooks")
	}

	return paginated(c, webhooks, total, page, size)
}

func (h *WebhookHandler) Delete(c *okapi.Context, req *DeleteWebhookRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("Insufficient workspace permissions", err)
	}
	wh, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, wh.UserID, wh.WorkspaceID) {
		return c.AbortNotFound("webhook not found")
	}

	if err := h.repo.Delete(wh.ID); err != nil {
		return c.AbortInternalServerError("failed to delete webhook")
	}

	h.audit.LogCtx(c, "webhook.deleted", "Webhook deleted: "+wh.URL, nil)

	return noContent(c)
}
