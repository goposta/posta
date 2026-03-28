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
	"crypto/rand"
	"encoding/hex"

	"github.com/jkaninda/okapi"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/audit"
	"github.com/goposta/posta/internal/storage/repositories"
)

type WebhookHandler struct {
	repo  *repositories.WebhookRepository
	audit *audit.Logger
}
type CreateWebhookRequest struct {
	Body struct {
		URL     string   `json:"url" required:"true" format:"uri"`
		Events  []string `json:"events" required:"true" minItems:"1"`
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
		return err
	}
	scope := getScope(c)

	// Validate event names
	validEvents := map[string]bool{"email.sent": true, "email.failed": true}
	for _, event := range req.Body.Events {
		if !validEvents[event] {
			return c.AbortBadRequest("invalid event: " + event + ". Valid events: email.sent, email.failed")
		}
	}

	// Generate a random signing secret for HMAC webhook signatures
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return c.AbortInternalServerError("failed to generate webhook secret", err)
	}
	secret := hex.EncodeToString(secretBytes)

	wh := &models.Webhook{
		UserID:      scope.UserID,
		WorkspaceID: scope.WorkspaceID,
		URL:     req.Body.URL,
		Events:  req.Body.Events,
		Filters: req.Body.Filters,
		Secret:  secret,
	}

	if err := h.repo.Create(wh); err != nil {
		return c.AbortInternalServerError("failed to create webhook", err)
	}

	h.audit.Log(scope.UserID, c.GetString("email"), c.RealIP(), "webhook.created", "Webhook created: "+req.Body.URL, nil)

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
		return err
	}
	wh, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, wh.UserID, wh.WorkspaceID) {
		return c.AbortNotFound("webhook not found")
	}

	if err := h.repo.Delete(wh.ID); err != nil {
		return c.AbortInternalServerError("failed to delete webhook")
	}

	h.audit.Log(wh.UserID, c.GetString("email"), c.RealIP(), "webhook.deleted", "Webhook deleted: "+wh.URL, nil)

	return noContent(c)
}
