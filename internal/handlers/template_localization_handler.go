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
	"github.com/jkaninda/posta/internal/services/email"
	"github.com/jkaninda/posta/internal/storage/repositories"
)

type TemplateLocalizationHandler struct {
	templateRepo     *repositories.TemplateRepository
	versionRepo      *repositories.TemplateVersionRepository
	localizationRepo *repositories.TemplateLocalizationRepository
	stylesheetRepo   *repositories.StyleSheetRepository
}
type ListLocalizationsRequest struct {
	TemplateID int `param:"id"`
	VersionID  int `param:"versionId"`
}
type CreateLocalizationRequest struct {
	TemplateID int `param:"id"`
	VersionID  int `param:"versionId"`
	Body       struct {
		Language        string `json:"language" required:"true"`
		SubjectTemplate string `json:"subject_template" required:"true"`
		HTMLTemplate    string `json:"html_template"`
		TextTemplate    string `json:"text_template"`
	} `json:"body"`
}
type UpdateLocalizationRequest struct {
	ID   int `param:"localizationId"`
	Body struct {
		SubjectTemplate *string `json:"subject_template"`
		HTMLTemplate    *string `json:"html_template"`
		TextTemplate    *string `json:"text_template"`
	} `json:"body"`
}
type DeleteLocalizationRequest struct {
	ID int `param:"localizationId"`
}
type PreviewLocalizationRequest struct {
	TemplateID int `param:"id"`
	VersionID  int `param:"versionId"`
	Body       struct {
		Language     string         `json:"language" required:"true"`
		TemplateData map[string]any `json:"template_data"`
	} `json:"body"`
}

func NewTemplateLocalizationHandler(
	templateRepo *repositories.TemplateRepository,
	versionRepo *repositories.TemplateVersionRepository,
	localizationRepo *repositories.TemplateLocalizationRepository,
	stylesheetRepo *repositories.StyleSheetRepository,
) *TemplateLocalizationHandler {
	return &TemplateLocalizationHandler{
		templateRepo:     templateRepo,
		versionRepo:      versionRepo,
		localizationRepo: localizationRepo,
		stylesheetRepo:   stylesheetRepo,
	}
}

func (h *TemplateLocalizationHandler) List(c *okapi.Context, req *ListLocalizationsRequest) error {
	userID := c.GetInt("user_id")

	tmpl, err := h.templateRepo.FindByID(uint(req.TemplateID))
	if err != nil || tmpl.UserID != uint(userID) {
		return c.AbortNotFound("template not found")
	}

	v, err := h.versionRepo.FindByID(uint(req.VersionID))
	if err != nil || v.TemplateID != tmpl.ID {
		return c.AbortNotFound("version not found")
	}

	localizations, err := h.localizationRepo.FindByVersionID(v.ID)
	if err != nil {
		return c.AbortInternalServerError("failed to list localizations")
	}

	return ok(c, localizations)
}

func (h *TemplateLocalizationHandler) Create(c *okapi.Context, req *CreateLocalizationRequest) error {
	userID := c.GetInt("user_id")

	tmpl, err := h.templateRepo.FindByID(uint(req.TemplateID))
	if err != nil || tmpl.UserID != uint(userID) {
		return c.AbortNotFound("template not found")
	}

	v, err := h.versionRepo.FindByID(uint(req.VersionID))
	if err != nil || v.TemplateID != tmpl.ID {
		return c.AbortNotFound("version not found")
	}

	l := &models.TemplateLocalization{
		VersionID:       v.ID,
		Language:        req.Body.Language,
		SubjectTemplate: req.Body.SubjectTemplate,
		HTMLTemplate:    req.Body.HTMLTemplate,
		TextTemplate:    req.Body.TextTemplate,
	}

	if err := h.localizationRepo.Create(l); err != nil {
		return c.AbortConflict("localization for this language already exists")
	}

	return created(c, l)
}

func (h *TemplateLocalizationHandler) Update(c *okapi.Context, req *UpdateLocalizationRequest) error {
	l, err := h.localizationRepo.FindByID(uint(req.ID))
	if err != nil {
		return c.AbortNotFound("localization not found")
	}

	// Verify ownership through the version → template chain
	v, err := h.versionRepo.FindByID(l.VersionID)
	if err != nil {
		return c.AbortNotFound("version not found")
	}
	tmpl, err := h.templateRepo.FindByID(v.TemplateID)
	if err != nil {
		return c.AbortNotFound("template not found")
	}
	userID := c.GetInt("user_id")
	if tmpl.UserID != uint(userID) {
		return c.AbortNotFound("localization not found")
	}

	if req.Body.SubjectTemplate != nil {
		l.SubjectTemplate = *req.Body.SubjectTemplate
	}
	if req.Body.HTMLTemplate != nil {
		l.HTMLTemplate = *req.Body.HTMLTemplate
	}
	if req.Body.TextTemplate != nil {
		l.TextTemplate = *req.Body.TextTemplate
	}

	now := time.Now()
	l.UpdatedAt = &now

	if err := h.localizationRepo.Update(l); err != nil {
		return c.AbortInternalServerError("failed to update localization")
	}

	return ok(c, l)
}

func (h *TemplateLocalizationHandler) Delete(c *okapi.Context, req *DeleteLocalizationRequest) error {
	l, err := h.localizationRepo.FindByID(uint(req.ID))
	if err != nil {
		return c.AbortNotFound("localization not found")
	}

	v, err := h.versionRepo.FindByID(l.VersionID)
	if err != nil {
		return c.AbortNotFound("version not found")
	}
	tmpl, err := h.templateRepo.FindByID(v.TemplateID)
	if err != nil {
		return c.AbortNotFound("template not found")
	}
	userID := c.GetInt("user_id")
	if tmpl.UserID != uint(userID) {
		return c.AbortNotFound("localization not found")
	}

	if err := h.localizationRepo.Delete(l.ID); err != nil {
		return c.AbortInternalServerError("failed to delete localization")
	}

	return noContent(c)
}

func (h *TemplateLocalizationHandler) Preview(c *okapi.Context, req *PreviewLocalizationRequest) error {
	userID := c.GetInt("user_id")

	tmpl, err := h.templateRepo.FindByID(uint(req.TemplateID))
	if err != nil || tmpl.UserID != uint(userID) {
		return c.AbortNotFound("template not found")
	}

	v, err := h.versionRepo.FindByID(uint(req.VersionID))
	if err != nil || v.TemplateID != tmpl.ID {
		return c.AbortNotFound("version not found")
	}

	l, err := h.localizationRepo.FindByVersionAndLanguage(v.ID, req.Body.Language)
	if err != nil {
		return c.AbortNotFound("localization not found for language: " + req.Body.Language)
	}

	data := req.Body.TemplateData
	if data == nil {
		data = map[string]any{}
	}

	// Build render input from the localization
	var css string
	if v.StyleSheet != nil {
		css = v.StyleSheet.CSS
	}
	input := &email.RenderInput{
		SubjectTemplate: l.SubjectTemplate,
		HTMLTemplate:    l.HTMLTemplate,
		TextTemplate:    l.TextTemplate,
		CSS:             css,
	}

	renderer := email.NewTemplateRenderer()
	rendered, err := renderer.Render(input, data)
	if err != nil {
		return c.AbortBadRequest("template render error: " + err.Error())
	}

	return ok(c, PreviewResult{
		Subject: rendered.Subject,
		HTML:    rendered.HTML,
		Text:    rendered.Text,
	})
}
