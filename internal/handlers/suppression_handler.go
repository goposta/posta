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
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
)

type SuppressionHandler struct {
	repo      *repositories.SuppressionRepository
	unsubRepo *repositories.UnsubscribeListRepository
}
type CreateSuppressionRequest struct {
	Body struct {
		Email  string `json:"email" required:"true" format:"email"`
		Reason string `json:"reason"`
		// ListID scopes the suppression to one UnsubscribeList. Omit for a global block.
		ListID *uint `json:"list_id,omitempty"`
	} `json:"body"`
}
type ListSuppressionsRequest struct {
	Page   int  `query:"page" default:"0"`
	Size   int  `query:"size" default:"20"`
	ListID uint `query:"list_id" default:"0"`
}
type DeleteSuppressionRequest struct {
	Body struct {
		Email  string `json:"email" required:"true" format:"email"`
		ListID *uint  `json:"list_id,omitempty"`
	} `json:"body"`
}

func NewSuppressionHandler(repo *repositories.SuppressionRepository, unsubRepo *repositories.UnsubscribeListRepository) *SuppressionHandler {
	return &SuppressionHandler{repo: repo, unsubRepo: unsubRepo}
}

func (h *SuppressionHandler) Create(c *okapi.Context, req *CreateSuppressionRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("Insufficient workspace permissions", err)
	}
	scope := getScope(c)

	// A list-scoped suppression must reference a list the caller owns.
	if req.Body.ListID != nil {
		if h.unsubRepo == nil {
			return c.AbortBadRequest("list-scoped suppression is not available")
		}
		if _, err := h.unsubRepo.FindByIDForScope(*req.Body.ListID, scope); err != nil {
			return c.AbortNotFound("unsubscribe list not found")
		}
	}

	suppression := &models.Suppression{
		UserID:      scope.UserID,
		WorkspaceID: scope.WorkspaceID,
		Email:       req.Body.Email,
		ListID:      req.Body.ListID,
		Kind:        models.SuppressionKindManual,
		Reason:      req.Body.Reason,
	}

	if err := h.repo.Create(suppression); err != nil {
		return c.AbortConflict("email already in suppression list")
	}

	return created(c, suppression)
}

func (h *SuppressionHandler) List(c *okapi.Context, req *ListSuppressionsRequest) error {
	page, size, offset := normalizePageParams(req.Page, req.Size)

	var listID *uint
	if req.ListID > 0 {
		listID = &req.ListID
	}

	suppressions, total, err := h.repo.FindByScopeFiltered(getScope(c), listID, size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list suppressions")
	}

	return paginated(c, suppressions, total, page, size)
}

func (h *SuppressionHandler) Delete(c *okapi.Context, req *DeleteSuppressionRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("Insufficient workspace permissions", err)
	}
	scope := getScope(c)

	if err := h.repo.Delete(scope, req.Body.Email, req.Body.ListID); err != nil {
		return c.AbortInternalServerError("failed to remove from suppression list")
	}

	return noContent(c)
}
