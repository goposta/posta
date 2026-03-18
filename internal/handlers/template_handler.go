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
	"fmt"
	"time"

	"github.com/jkaninda/okapi"
	"github.com/jkaninda/posta/internal/config"
	"github.com/jkaninda/posta/internal/models"
	"github.com/jkaninda/posta/internal/services/email"
	"github.com/jkaninda/posta/internal/storage/repositories"
)

type TemplateHandler struct {
	repo             *repositories.TemplateRepository
	stylesheetRepo   *repositories.StyleSheetRepository
	versionRepo      *repositories.TemplateVersionRepository
	localizationRepo *repositories.TemplateLocalizationRepository
	emailService     *email.Service
}
type CreateTemplateRequest struct {
	Body struct {
		Name            string `json:"name" required:"true"`
		SampleData      string `json:"sample_data"`
		DefaultLanguage string `json:"default_language"`
		Description     string `json:"description"`
	} `json:"body"`
}
type UpdateTemplateRequest struct {
	ID   int `param:"id"`
	Body struct {
		Name            string  `json:"name"`
		SampleData      *string `json:"sample_data"`
		DefaultLanguage string  `json:"default_language"`
		Description     *string `json:"description"`
	} `json:"body"`
}
type DeleteTemplateRequest struct {
	ID int `param:"id"`
}
type PreviewTemplateRequest struct {
	Body struct {
		SubjectTemplate string         `json:"subject_template" required:"true"`
		HTMLTemplate    string         `json:"html_template"`
		TextTemplate    string         `json:"text_template"`
		StyleSheetID    *uint          `json:"stylesheet_id"`
		TemplateData    map[string]any `json:"template_data"`
	} `json:"body"`
}
type PreviewResult struct {
	Subject string `json:"subject"`
	HTML    string `json:"html"`
	Text    string `json:"text"`
}
type SendTestRequest struct {
	ID   int                   `param:"id"`
	Body email.SendTestRequest `json:"body"`
}

func NewTemplateHandler(repo *repositories.TemplateRepository, ssRepo *repositories.StyleSheetRepository, versionRepo *repositories.TemplateVersionRepository, localizationRepo *repositories.TemplateLocalizationRepository, emailService *email.Service) *TemplateHandler {
	return &TemplateHandler{repo: repo, stylesheetRepo: ssRepo, versionRepo: versionRepo, localizationRepo: localizationRepo, emailService: emailService}
}

func (h *TemplateHandler) Create(c *okapi.Context, req *CreateTemplateRequest) error {
	userID := c.GetInt("user_id")

	defaultLang := req.Body.DefaultLanguage
	if defaultLang == "" {
		defaultLang = "en"
	}

	tmpl := &models.Template{
		UserID:          uint(userID),
		Name:            req.Body.Name,
		DefaultLanguage: defaultLang,
		Description:     req.Body.Description,
		SampleData:      req.Body.SampleData,
	}

	if err := h.repo.Create(tmpl); err != nil {
		return c.AbortConflict("template name already exists")
	}

	// Create default version v1 and set it as active
	v := &models.TemplateVersion{
		TemplateID: tmpl.ID,
		Version:    1,
		SampleData: req.Body.SampleData,
	}
	if err := h.versionRepo.Create(v); err != nil {
		return c.AbortInternalServerError("failed to create default version")
	}

	tmpl.ActiveVersionID = &v.ID
	if err := h.repo.Update(tmpl); err != nil {
		return c.AbortInternalServerError("failed to activate default version")
	}

	return created(c, tmpl)
}

func (h *TemplateHandler) Update(c *okapi.Context, req *UpdateTemplateRequest) error {
	userID := c.GetInt("user_id")

	tmpl, err := h.repo.FindByID(uint(req.ID))
	if err != nil || tmpl.UserID != uint(userID) {
		return c.AbortNotFound("template not found")
	}

	if req.Body.Name != "" {
		tmpl.Name = req.Body.Name
	}
	if req.Body.SampleData != nil {
		tmpl.SampleData = *req.Body.SampleData
	}
	if req.Body.DefaultLanguage != "" {
		tmpl.DefaultLanguage = req.Body.DefaultLanguage
	}
	if req.Body.Description != nil {
		tmpl.Description = *req.Body.Description
	}

	now := time.Now()
	tmpl.UpdatedAt = &now

	if err := h.repo.Update(tmpl); err != nil {
		return c.AbortInternalServerError("failed to update template")
	}

	return ok(c, tmpl)
}

func (h *TemplateHandler) List(c *okapi.Context, req *ListRequest) error {
	userID := c.GetInt("user_id")
	page, size, offset := normalizePageParams(req.Page, req.Size)

	templates, total, err := h.repo.FindByUserID(uint(userID), size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list templates")
	}

	return paginated(c, templates, total, page, size)
}

func (h *TemplateHandler) Delete(c *okapi.Context, req *DeleteTemplateRequest) error {
	userID := c.GetInt("user_id")

	tmpl, err := h.repo.FindByID(uint(req.ID))
	if err != nil || tmpl.UserID != uint(userID) {
		return c.AbortNotFound("template not found")
	}

	if err := h.repo.Delete(tmpl.ID); err != nil {
		return c.AbortInternalServerError("failed to delete template")
	}

	return noContent(c)
}

func (h *TemplateHandler) Preview(c *okapi.Context, req *PreviewTemplateRequest) error {
	renderer := email.NewTemplateRenderer()

	data := req.Body.TemplateData
	if data == nil {
		data = map[string]any{}
	}

	input := &email.RenderInput{
		SubjectTemplate: req.Body.SubjectTemplate,
		HTMLTemplate:    req.Body.HTMLTemplate,
		TextTemplate:    req.Body.TextTemplate,
	}

	// Resolve the linked stylesheet for preview
	if req.Body.StyleSheetID != nil && *req.Body.StyleSheetID > 0 {
		ss, err := h.stylesheetRepo.FindByID(*req.Body.StyleSheetID)
		if err != nil {
			return c.AbortNotFound("stylesheet not found")
		}
		input.CSS = ss.CSS
	}

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

func (h *TemplateHandler) SendTest(c *okapi.Context, req *SendTestRequest) error {
	userID := c.GetInt("user_id")
	userEmail := c.GetString("email")

	tmpl, err := h.repo.FindByID(uint(req.ID))
	if err != nil || tmpl.UserID != uint(userID) {
		return c.AbortNotFound("template not found")
	}

	resp, err := h.emailService.SendTestByTemplateID(c.Request().Context(), uint(userID), userEmail, tmpl.ID, &req.Body)
	if err != nil {
		return c.AbortBadRequest(err.Error())
	}

	return ok(c, resp)
}

// --- Export / Import ---

type ExportTemplateRequest struct {
	ID int `param:"id"`
}

type ExportLocalization struct {
	Language        string `json:"language"`
	SubjectTemplate string `json:"subject_template"`
	HTMLTemplate    string `json:"html_template"`
	TextTemplate    string `json:"text_template"`
}

type ExportVersion struct {
	Version       int                  `json:"version"`
	SampleData    string               `json:"sample_data"`
	IsActive      bool                 `json:"is_active"`
	Localizations []ExportLocalization `json:"localizations"`
}

type TemplateExport struct {
	PostaVersion    string          `json:"posta_version"`
	ExportedAt      string          `json:"exported_at"`
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	DefaultLanguage string          `json:"default_language"`
	SampleData      string          `json:"sample_data"`
	Versions        []ExportVersion `json:"versions"`
}

type ImportTemplateRequest struct {
	Body TemplateExport `json:"body"`
}

func (h *TemplateHandler) Export(c *okapi.Context, req *ExportTemplateRequest) error {
	userID := c.GetInt("user_id")

	tmpl, err := h.repo.FindByID(uint(req.ID))
	if err != nil || tmpl.UserID != uint(userID) {
		return c.AbortNotFound("template not found")
	}

	versions, err := h.versionRepo.FindByTemplateID(tmpl.ID)
	if err != nil {
		return c.AbortInternalServerError("failed to load versions")
	}

	exportVersions := make([]ExportVersion, 0, len(versions))
	for _, v := range versions {
		localizations, err := h.localizationRepo.FindByVersionID(v.ID)
		if err != nil {
			return c.AbortInternalServerError("failed to load localizations")
		}

		exportLocs := make([]ExportLocalization, 0, len(localizations))
		for _, l := range localizations {
			exportLocs = append(exportLocs, ExportLocalization{
				Language:        l.Language,
				SubjectTemplate: l.SubjectTemplate,
				HTMLTemplate:    l.HTMLTemplate,
				TextTemplate:    l.TextTemplate,
			})
		}

		isActive := tmpl.ActiveVersionID != nil && *tmpl.ActiveVersionID == v.ID
		exportVersions = append(exportVersions, ExportVersion{
			Version:       v.Version,
			SampleData:    v.SampleData,
			IsActive:      isActive,
			Localizations: exportLocs,
		})
	}

	export := TemplateExport{
		PostaVersion:    config.Version,
		ExportedAt:      time.Now().UTC().Format(time.RFC3339),
		Name:            tmpl.Name,
		Description:     tmpl.Description,
		DefaultLanguage: tmpl.DefaultLanguage,
		SampleData:      tmpl.SampleData,
		Versions:        exportVersions,
	}

	return ok(c, export)
}

func (h *TemplateHandler) Import(c *okapi.Context, req *ImportTemplateRequest) error {
	userID := c.GetInt("user_id")
	data := req.Body

	if data.Name == "" {
		return c.AbortBadRequest("template name is required")
	}

	defaultLang := data.DefaultLanguage
	if defaultLang == "" {
		defaultLang = "en"
	}

	tmpl := &models.Template{
		UserID:          uint(userID),
		Name:            data.Name,
		DefaultLanguage: defaultLang,
		Description:     data.Description,
		SampleData:      data.SampleData,
	}

	if err := h.repo.Create(tmpl); err != nil {
		return c.AbortConflict("template name already exists")
	}

	var activeVersionDBID *uint

	for _, ev := range data.Versions {
		nextVersion, err := h.versionRepo.NextVersion(tmpl.ID)
		if err != nil {
			return c.AbortInternalServerError("failed to determine next version")
		}

		v := &models.TemplateVersion{
			TemplateID: tmpl.ID,
			Version:    nextVersion,
			SampleData: ev.SampleData,
		}
		if err := h.versionRepo.Create(v); err != nil {
			return c.AbortInternalServerError(fmt.Sprintf("failed to create version %d", ev.Version))
		}

		if ev.IsActive {
			activeVersionDBID = &v.ID
		}

		for _, el := range ev.Localizations {
			l := &models.TemplateLocalization{
				VersionID:       v.ID,
				Language:        el.Language,
				SubjectTemplate: el.SubjectTemplate,
				HTMLTemplate:    el.HTMLTemplate,
				TextTemplate:    el.TextTemplate,
			}
			if err := h.localizationRepo.Create(l); err != nil {
				return c.AbortInternalServerError(fmt.Sprintf("failed to create localization %s for version %d", el.Language, ev.Version))
			}
		}
	}

	// If no versions were imported, create a default v1
	if len(data.Versions) == 0 {
		v := &models.TemplateVersion{
			TemplateID: tmpl.ID,
			Version:    1,
			SampleData: data.SampleData,
		}
		if err := h.versionRepo.Create(v); err != nil {
			return c.AbortInternalServerError("failed to create default version")
		}
		activeVersionDBID = &v.ID
	}

	if activeVersionDBID != nil {
		tmpl.ActiveVersionID = activeVersionDBID
		if err := h.repo.Update(tmpl); err != nil {
			return c.AbortInternalServerError("failed to activate version")
		}
	}

	return created(c, tmpl)
}
