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
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
)

// A send references a list by id so its one-click link suppresses the recipient on that list only.
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
