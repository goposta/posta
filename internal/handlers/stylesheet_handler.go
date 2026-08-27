// SPDX-FileCopyrightText: 2026 Jonas Kaninda
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
		return c.AbortForbidden("Insufficient workspace permissions", err)
	}
	scope := getScope(c)

	ss := &models.StyleSheet{
		UserID:      scope.UserID,
		WorkspaceID: scope.WorkspaceID,
		Name:        req.Body.Name,
		CSS:         req.Body.CSS,
	}

	if err := h.repo.Create(ss); err != nil {
		return c.AbortConflict("stylesheet name already exists")
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
