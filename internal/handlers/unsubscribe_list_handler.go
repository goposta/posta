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
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
)

// UnsubscribeListHandler is the scoped CRUD API for transactional unsubscribe
// lists. A send references one of these by id so Posta can mint a one-click link
// whose click suppresses the recipient on that list only.
type UnsubscribeListHandler struct {
	repo *repositories.UnsubscribeListRepository
}

func NewUnsubscribeListHandler(repo *repositories.UnsubscribeListRepository) *UnsubscribeListHandler {
	return &UnsubscribeListHandler{repo: repo}
}

type CreateUnsubscribeListRequest struct {
	Body struct {
		Name        string `json:"name" required:"true"`
		PublicName  string `json:"public_name"`
		Description string `json:"description"`
	} `json:"body"`
}

type UpdateUnsubscribeListRequest struct {
	ID   int `param:"id"`
	Body struct {
		Name        string `json:"name"`
		PublicName  string `json:"public_name"`
		Description string `json:"description"`
		Active      *bool  `json:"active"`
	} `json:"body"`
}

type GetUnsubscribeListRequest struct {
	ID int `param:"id"`
}

type DeleteUnsubscribeListRequest struct {
	ID int `param:"id"`
}

func (h *UnsubscribeListHandler) Create(c *okapi.Context, req *CreateUnsubscribeListRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("Insufficient workspace permissions", err)
	}
	scope := getScope(c)

	list := &models.UnsubscribeList{
		UserID:      scope.UserID,
		WorkspaceID: scope.WorkspaceID,
		Name:        req.Body.Name,
		PublicName:  req.Body.PublicName,
		Description: req.Body.Description,
		Active:      true,
	}
	if err := h.repo.Create(list); err != nil {
		return c.AbortConflict("an unsubscribe list with that name already exists")
	}
	return created(c, list)
}

func (h *UnsubscribeListHandler) List(c *okapi.Context, req *ListRequest) error {
	page, size, offset := normalizePageParams(req.Page, req.Size)
	lists, total, err := h.repo.FindByScope(getScope(c), size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list unsubscribe lists")
	}
	return paginated(c, lists, total, page, size)
}

func (h *UnsubscribeListHandler) Get(c *okapi.Context, req *GetUnsubscribeListRequest) error {
	list, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, list.UserID, list.WorkspaceID) {
		return c.AbortNotFound("unsubscribe list not found")
	}
	return ok(c, list)
}

func (h *UnsubscribeListHandler) Update(c *okapi.Context, req *UpdateUnsubscribeListRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("Insufficient workspace permissions", err)
	}
	list, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, list.UserID, list.WorkspaceID) {
		return c.AbortNotFound("unsubscribe list not found")
	}

	if req.Body.Name != "" {
		list.Name = req.Body.Name
	}
	if req.Body.PublicName != "" {
		list.PublicName = req.Body.PublicName
	}
	if req.Body.Description != "" {
		list.Description = req.Body.Description
	}
	if req.Body.Active != nil {
		list.Active = *req.Body.Active
	}
	now := time.Now()
	list.UpdatedAt = &now

	if err := h.repo.Update(list); err != nil {
		return c.AbortInternalServerError("failed to update unsubscribe list")
	}
	return ok(c, list)
}

func (h *UnsubscribeListHandler) Delete(c *okapi.Context, req *DeleteUnsubscribeListRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("Insufficient workspace permissions", err)
	}
	list, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, list.UserID, list.WorkspaceID) {
		return c.AbortNotFound("unsubscribe list not found")
	}
	if err := h.repo.Delete(list.ID); err != nil {
		return c.AbortInternalServerError("failed to delete unsubscribe list")
	}
	return noContent(c)
}
