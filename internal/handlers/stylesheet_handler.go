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

type StyleSheetHandler struct {
	repo *repositories.StyleSheetRepository
}
type CreateStyleSheetRequest struct {
	Body struct {
		Name        string `json:"name" doc:"The stylesheet's name, as typed: \"Brand Styles\" is stored as the identifier brand-styles, with \"Brand Styles\" as display_name. It cannot change later."`
		DisplayName string `json:"display_name" doc:"The stylesheet's label. Defaults to name."`
		CSS         string `json:"css"`
	} `json:"body"`
}
type UpdateStyleSheetRequest struct {
	ID   int `param:"id"`
	Body struct {
		Name        string `json:"name"`
		DisplayName string `json:"display_name"`
		CSS         string `json:"css"`
	} `json:"body"`
}
type DeleteStyleSheetRequest struct {
	ID int `param:"id"`
}

func NewStyleSheetHandler(repo *repositories.StyleSheetRepository) *StyleSheetHandler {
	return &StyleSheetHandler{repo: repo}
}

func (h *StyleSheetHandler) Create(c *okapi.Context, req *CreateStyleSheetRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("Insufficient workspace permissions", err)
	}
	scope := getScope(c)

	name, displayName, err := resolveNames(req.Body.Name, req.Body.DisplayName, "")
	if err != nil {
		return c.AbortBadRequest(err.Error())
	}
	ss := &models.StyleSheet{
		UserID:      scope.UserID,
		WorkspaceID: scope.WorkspaceID,
		Name:        name,
		DisplayName: displayName,
		CSS:         req.Body.CSS,
	}

	if err := h.repo.Create(ss); err != nil {
		return c.AbortConflict("a stylesheet named \"" + name + "\" already exists")
	}

	return created(c, ss)
}

func (h *StyleSheetHandler) Update(c *okapi.Context, req *UpdateStyleSheetRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("Insufficient workspace permissions", err)
	}
	ss, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, ss.UserID, ss.WorkspaceID) {
		return c.AbortNotFound("stylesheet not found")
	}

	displayName, err := resolveRename(req.Body.Name, req.Body.DisplayName, "", ss.Name, "stylesheet")
	if err != nil {
		return c.AbortBadRequest(err.Error())
	}
	if displayName != "" {
		ss.DisplayName = displayName
	}
	if req.Body.CSS != "" {
		ss.CSS = req.Body.CSS
	}

	now := time.Now()
	ss.UpdatedAt = &now

	if err := h.repo.Update(ss); err != nil {
		return c.AbortInternalServerError("failed to update stylesheet")
	}

	return ok(c, ss)
}

func (h *StyleSheetHandler) Delete(c *okapi.Context, req *DeleteStyleSheetRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("Insufficient workspace permissions", err)
	}
	ss, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, ss.UserID, ss.WorkspaceID) {
		return c.AbortNotFound("stylesheet not found")
	}

	if err := h.repo.Delete(ss.ID); err != nil {
		return c.AbortInternalServerError("failed to delete stylesheet")
	}

	return noContent(c)
}

func (h *StyleSheetHandler) List(c *okapi.Context, req *ListRequest) error {
	page, size, offset := normalizePageParams(req.Page, req.Size)

	sheets, total, err := h.repo.FindByScope(getScope(c), size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list stylesheets")
	}

	return paginated(c, sheets, total, page, size)
}
