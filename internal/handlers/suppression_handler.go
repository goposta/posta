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
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/storage/repositories"
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
	if err := requireEdit(c); err != nil {
		return err
	}
	scope := getScope(c)

	suppression := &models.Suppression{
		UserID:      scope.UserID,
		WorkspaceID: scope.WorkspaceID,
		Email:       req.Body.Email,
		Reason:      req.Body.Reason,
	}

	if err := h.repo.Create(suppression); err != nil {
		return c.AbortConflict("email already in suppression list")
	}

	return created(c, suppression)
}

func (h *SuppressionHandler) List(c *okapi.Context, req *ListRequest) error {
	page, size, offset := normalizePageParams(req.Page, req.Size)

	suppressions, total, err := h.repo.FindByScope(getScope(c), size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list suppressions")
	}

	return paginated(c, suppressions, total, page, size)
}

func (h *SuppressionHandler) Delete(c *okapi.Context, req *DeleteSuppressionRequest) error {
	if err := requireEdit(c); err != nil {
		return err
	}
	scope := getScope(c)

	if err := h.repo.Delete(scope, req.Body.Email); err != nil {
		return c.AbortInternalServerError("failed to remove from suppression list")
	}

	return noContent(c)
}
