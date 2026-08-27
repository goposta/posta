// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package handlers

import (
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
)

type WebhookDeliveryHandler struct {
	repo *repositories.WebhookDeliveryRepository
}

func NewWebhookDeliveryHandler(repo *repositories.WebhookDeliveryRepository) *WebhookDeliveryHandler {
	return &WebhookDeliveryHandler{repo: repo}
}

func (h *WebhookDeliveryHandler) List(c *okapi.Context, req *ListRequest) error {
	page, size, offset := normalizePageParams(req.Page, req.Size)

	deliveries, total, err := h.repo.FindByScope(getScope(c), size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list webhook deliveries")
	}

	return paginated(c, deliveries, total, page, size)
}
