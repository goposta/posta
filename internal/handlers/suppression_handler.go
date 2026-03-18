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
	"github.com/jkaninda/posta/internal/models"
	"github.com/jkaninda/posta/internal/storage/repositories"
)

type SuppressionHandler struct {
	repo *repositories.SuppressionRepository
}
type CreateSuppressionRequest struct {
	Body struct {
		Email  string `json:"email" required:"true" format:"email"`
		Reason string `json:"reason"`
	} `json:"body"`
}
type DeleteSuppressionRequest struct {
	Body struct {
		Email string `json:"email" required:"true" format:"email"`
	} `json:"body"`
}

func NewSuppressionHandler(repo *repositories.SuppressionRepository) *SuppressionHandler {
	return &SuppressionHandler{repo: repo}
}

func (h *SuppressionHandler) Create(c *okapi.Context, req *CreateSuppressionRequest) error {
	userID := c.GetInt("user_id")

	suppression := &models.Suppression{
		UserID: uint(userID),
		Email:  req.Body.Email,
		Reason: req.Body.Reason,
	}

	if err := h.repo.Create(suppression); err != nil {
		return c.AbortConflict("email already in suppression list")
	}

	return created(c, suppression)
}

func (h *SuppressionHandler) List(c *okapi.Context, req *ListRequest) error {
	userID := c.GetInt("user_id")
	page, size, offset := normalizePageParams(req.Page, req.Size)

	suppressions, total, err := h.repo.FindByUserID(uint(userID), size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list suppressions")
	}

	return paginated(c, suppressions, total, page, size)
}

func (h *SuppressionHandler) Delete(c *okapi.Context, req *DeleteSuppressionRequest) error {
	userID := c.GetInt("user_id")

	if err := h.repo.Delete(uint(userID), req.Body.Email); err != nil {
		return c.AbortInternalServerError("failed to remove from suppression list")
	}

	return noContent(c)
}
