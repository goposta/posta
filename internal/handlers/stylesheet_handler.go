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
	"github.com/jkaninda/posta/internal/models"
	"github.com/jkaninda/posta/internal/storage/repositories"
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
	userID := c.GetInt("user_id")

	ss := &models.StyleSheet{
		UserID: uint(userID),
		Name:   req.Body.Name,
		CSS:    req.Body.CSS,
	}

	if err := h.repo.Create(ss); err != nil {
		return c.AbortConflict("stylesheet name already exists")
	}

	return created(c, ss)
}

func (h *StyleSheetHandler) Update(c *okapi.Context, req *UpdateStyleSheetRequest) error {
	userID := c.GetInt("user_id")

	ss, err := h.repo.FindByID(uint(req.ID))
	if err != nil || ss.UserID != uint(userID) {
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
	userID := c.GetInt("user_id")

	ss, err := h.repo.FindByID(uint(req.ID))
	if err != nil || ss.UserID != uint(userID) {
		return c.AbortNotFound("stylesheet not found")
	}

	if err := h.repo.Delete(ss.ID); err != nil {
		return c.AbortInternalServerError("failed to delete stylesheet")
	}

	return noContent(c)
}

func (h *StyleSheetHandler) List(c *okapi.Context, req *ListRequest) error {
	userID := c.GetInt("user_id")
	page, size, offset := normalizePageParams(req.Page, req.Size)

	sheets, total, err := h.repo.FindByUserID(uint(userID), size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list stylesheets")
	}

	return paginated(c, sheets, total, page, size)
}
