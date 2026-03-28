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

type LanguageHandler struct {
	repo *repositories.LanguageRepository
}
type CreateLanguageRequest struct {
	Body struct {
		Code      string `json:"code" required:"true"`
		Name      string `json:"name" required:"true"`
		IsDefault bool   `json:"is_default"`
	} `json:"body"`
}
type UpdateLanguageRequest struct {
	ID   int `param:"id"`
	Body struct {
		Code      string `json:"code"`
		Name      string `json:"name"`
		IsDefault *bool  `json:"is_default"`
	} `json:"body"`
}
type DeleteLanguageRequest struct {
	ID int `param:"id"`
}

func NewLanguageHandler(repo *repositories.LanguageRepository) *LanguageHandler {
	return &LanguageHandler{repo: repo}
}

func (h *LanguageHandler) Create(c *okapi.Context, req *CreateLanguageRequest) error {
	if err := requireEdit(c); err != nil {
		return err
	}
	scope := getScope(c)

	// If marking as default, clear existing default first
	if req.Body.IsDefault {
		_ = h.repo.ClearDefault(scope)
	}

	l := &models.Language{
		UserID:      scope.UserID,
		WorkspaceID: scope.WorkspaceID,
		Code:        req.Body.Code,
		Name:        req.Body.Name,
		IsDefault:   req.Body.IsDefault,
	}

	if err := h.repo.Create(l); err != nil {
		return c.AbortConflict("language code already exists")
	}

	return created(c, l)
}

func (h *LanguageHandler) Update(c *okapi.Context, req *UpdateLanguageRequest) error {
	if err := requireEdit(c); err != nil {
		return err
	}
	l, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, l.UserID, l.WorkspaceID) {
		return c.AbortNotFound("language not found")
	}

	if req.Body.Code != "" {
		l.Code = req.Body.Code
	}
	if req.Body.Name != "" {
		l.Name = req.Body.Name
	}

	// Handle default flag change
	if req.Body.IsDefault != nil {
		if *req.Body.IsDefault && !l.IsDefault {
			// Setting as default — clear others first
			scope := getScope(c)
			_ = h.repo.ClearDefault(scope)
		}
		l.IsDefault = *req.Body.IsDefault
	}

	if err := h.repo.Update(l); err != nil {
		return c.AbortInternalServerError("failed to update language")
	}

	return ok(c, l)
}

func (h *LanguageHandler) Delete(c *okapi.Context, req *DeleteLanguageRequest) error {
	if err := requireEdit(c); err != nil {
		return err
	}
	l, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, l.UserID, l.WorkspaceID) {
		return c.AbortNotFound("language not found")
	}

	if err := h.repo.Delete(l.ID); err != nil {
		return c.AbortInternalServerError("failed to delete language")
	}

	return noContent(c)
}

func (h *LanguageHandler) List(c *okapi.Context, req *ListRequest) error {
	page, size, offset := normalizePageParams(req.Page, req.Size)

	languages, total, err := h.repo.FindByScope(getScope(c), size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list languages")
	}

	return paginated(c, languages, total, page, size)
}
