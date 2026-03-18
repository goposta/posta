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

type TemplateVersionHandler struct {
	templateRepo *repositories.TemplateRepository
	versionRepo  *repositories.TemplateVersionRepository
}
type ListVersionsRequest struct {
	TemplateID int `param:"id"`
}
type CreateVersionRequest struct {
	TemplateID int `param:"id"`
	Body       struct {
		StyleSheetID *uint  `json:"stylesheet_id"`
		SampleData   string `json:"sample_data"`
	} `json:"body"`
}
type UpdateVersionRequest struct {
	TemplateID int `param:"id"`
	VersionID  int `param:"versionId"`
	Body       struct {
		StyleSheetID *uint `json:"stylesheet_id"`
	} `json:"body"`
}
type ActivateVersionRequest struct {
	TemplateID int `param:"id"`
	VersionID  int `param:"versionId"`
}
type DeleteVersionRequest struct {
	TemplateID int `param:"id"`
	VersionID  int `param:"versionId"`
}

func NewTemplateVersionHandler(
	templateRepo *repositories.TemplateRepository,
	versionRepo *repositories.TemplateVersionRepository,
) *TemplateVersionHandler {
	return &TemplateVersionHandler{
		templateRepo: templateRepo,
		versionRepo:  versionRepo,
	}
}

func (h *TemplateVersionHandler) List(c *okapi.Context, req *ListVersionsRequest) error {
	userID := c.GetInt("user_id")

	tmpl, err := h.templateRepo.FindByID(uint(req.TemplateID))
	if err != nil || tmpl.UserID != uint(userID) {
		return c.AbortNotFound("template not found")
	}

	versions, err := h.versionRepo.FindByTemplateID(tmpl.ID)
	if err != nil {
		return c.AbortInternalServerError("failed to list versions")
	}

	return ok(c, versions)
}

func (h *TemplateVersionHandler) Create(c *okapi.Context, req *CreateVersionRequest) error {
	userID := c.GetInt("user_id")

	tmpl, err := h.templateRepo.FindByID(uint(req.TemplateID))
	if err != nil || tmpl.UserID != uint(userID) {
		return c.AbortNotFound("template not found")
	}

	nextVersion, err := h.versionRepo.NextVersion(tmpl.ID)
	if err != nil {
		return c.AbortInternalServerError("failed to determine next version")
	}

	v := &models.TemplateVersion{
		TemplateID:   tmpl.ID,
		Version:      nextVersion,
		StyleSheetID: req.Body.StyleSheetID,
		SampleData:   req.Body.SampleData,
	}

	if err := h.versionRepo.Create(v); err != nil {
		return c.AbortInternalServerError("failed to create version")
	}

	v, _ = h.versionRepo.FindByID(v.ID)
	return created(c, v)
}

func (h *TemplateVersionHandler) Update(c *okapi.Context, req *UpdateVersionRequest) error {
	userID := c.GetInt("user_id")

	tmpl, err := h.templateRepo.FindByID(uint(req.TemplateID))
	if err != nil || tmpl.UserID != uint(userID) {
		return c.AbortNotFound("template not found")
	}

	v, err := h.versionRepo.FindByID(uint(req.VersionID))
	if err != nil || v.TemplateID != tmpl.ID {
		return c.AbortNotFound("version not found")
	}

	v.StyleSheetID = req.Body.StyleSheetID

	if err := h.versionRepo.Update(v); err != nil {
		return c.AbortInternalServerError("failed to update version")
	}

	// Reload to get the stylesheet association
	v, _ = h.versionRepo.FindByID(v.ID)
	return ok(c, v)
}

func (h *TemplateVersionHandler) Activate(c *okapi.Context, req *ActivateVersionRequest) error {
	userID := c.GetInt("user_id")

	tmpl, err := h.templateRepo.FindByID(uint(req.TemplateID))
	if err != nil || tmpl.UserID != uint(userID) {
		return c.AbortNotFound("template not found")
	}

	v, err := h.versionRepo.FindByID(uint(req.VersionID))
	if err != nil || v.TemplateID != tmpl.ID {
		return c.AbortNotFound("version not found")
	}

	vID := v.ID
	tmpl.ActiveVersionID = &vID
	now := time.Now()
	tmpl.UpdatedAt = &now

	if err := h.templateRepo.Update(tmpl); err != nil {
		return c.AbortInternalServerError("failed to activate version")
	}

	return ok(c, tmpl)
}

func (h *TemplateVersionHandler) Delete(c *okapi.Context, req *DeleteVersionRequest) error {
	userID := c.GetInt("user_id")

	tmpl, err := h.templateRepo.FindByID(uint(req.TemplateID))
	if err != nil || tmpl.UserID != uint(userID) {
		return c.AbortNotFound("template not found")
	}

	v, err := h.versionRepo.FindByID(uint(req.VersionID))
	if err != nil || v.TemplateID != tmpl.ID {
		return c.AbortNotFound("version not found")
	}

	// Prevent deleting the active version
	if tmpl.ActiveVersionID != nil && *tmpl.ActiveVersionID == v.ID {
		return c.AbortBadRequest("cannot delete the active version")
	}

	if err := h.versionRepo.Delete(v.ID); err != nil {
		return c.AbortInternalServerError("failed to delete version")
	}

	return noContent(c)
}
