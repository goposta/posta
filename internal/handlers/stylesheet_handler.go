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

	"github.com/jkaninda/okapi"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/storage/repositories"
)

type StyleSheetHandler struct {
	repo *repositories.StyleSheetRepository
}
type CreateStyleSheetRequest struct {
	Body struct {
		Name string `json:"name" required:"true"`
		CSS  string `json:"css"`
	} `json:"body"`
}
type UpdateStyleSheetRequest struct {
	ID   int `param:"id"`
	Body struct {
		Name string `json:"name"`
		CSS  string `json:"css"`
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
		return err
	}
	scope := getScope(c)

	ss := &models.StyleSheet{
		UserID:      scope.UserID,
		WorkspaceID: scope.WorkspaceID,
		Name:        req.Body.Name,
		CSS:    req.Body.CSS,
	}

	if err := h.repo.Create(ss); err != nil {
		return c.AbortConflict("stylesheet name already exists")
	}

	return created(c, ss)
}

func (h *StyleSheetHandler) Update(c *okapi.Context, req *UpdateStyleSheetRequest) error {
	if err := requireEdit(c); err != nil {
		return err
	}
	ss, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, ss.UserID, ss.WorkspaceID) {
		return c.AbortNotFound("stylesheet not found")
	}

	if req.Body.Name != "" {
		ss.Name = req.Body.Name
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
		return err
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
