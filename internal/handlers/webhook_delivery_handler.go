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
	"github.com/jkaninda/okapi"
	"github.com/goposta/posta/internal/storage/repositories"
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
