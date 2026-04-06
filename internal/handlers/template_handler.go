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
	"io"
	"strings"
	"time"

	"github.com/goposta/posta/internal/config"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/email"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
)

type TemplateHandler struct {
	repo             *repositories.TemplateRepository
	stylesheetRepo   *repositories.StyleSheetRepository
	versionRepo      *repositories.TemplateVersionRepository
	localizationRepo *repositories.TemplateLocalizationRepository
	languageRepo     *repositories.LanguageRepository
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
type ListTemplatesRequest struct {
	Page   int    `query:"page" default:"0"`
	Size   int    `query:"size" default:"20"`
	Search string `query:"search"`
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

func NewTemplateHandler(repo *repositories.TemplateRepository, ssRepo *repositories.StyleSheetRepository, versionRepo *repositories.TemplateVersionRepository, localizationRepo *repositories.TemplateLocalizationRepository, languageRepo *repositories.LanguageRepository, emailService *email.Service) *TemplateHandler {
	return &TemplateHandler{repo: repo, stylesheetRepo: ssRepo, versionRepo: versionRepo, localizationRepo: localizationRepo, languageRepo: languageRepo, emailService: emailService}
}

func (h *TemplateHandler) Create(c *okapi.Context, req *CreateTemplateRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("Insufficient workspace permissions", err)
	}
	scope := getScope(c)

	defaultLang := req.Body.DefaultLanguage
	if defaultLang == "" {
		// Use the scope's default language if one is configured
		if defLang, err := h.languageRepo.FindDefault(scope); err == nil {
			defaultLang = defLang.Code
		} else {
			defaultLang = "en"
		}
	}

	tmpl := &models.Template{
		UserID:          scope.UserID,
		WorkspaceID:     scope.WorkspaceID,
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
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("Insufficient workspace permissions", err)
	}
	tmpl, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, tmpl.UserID, tmpl.WorkspaceID) {
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

func (h *TemplateHandler) List(c *okapi.Context, req *ListTemplatesRequest) error {
	page, size, offset := normalizePageParams(req.Page, req.Size)

	templates, total, err := h.repo.FindByScope(getScope(c), req.Search, size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list templates")
	}

	return paginated(c, templates, total, page, size)
}

func (h *TemplateHandler) Delete(c *okapi.Context, req *DeleteTemplateRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("Insufficient workspace permissions", err)
	}
	tmpl, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, tmpl.UserID, tmpl.WorkspaceID) {
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
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("Insufficient workspace permissions", err)
	}
	userEmail := c.GetString("email")
	scope := getScope(c)

	tmpl, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, tmpl.UserID, tmpl.WorkspaceID) {
		return c.AbortNotFound("template not found")
	}

	resp, err := h.emailService.SendTestByTemplateID(c.Request().Context(), scope.UserID, scope.WorkspaceID, userEmail, tmpl.ID, &req.Body)
	if err != nil {
		return c.AbortBadRequest(err.Error())
	}

	return ok(c, resp)
}

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
	tmpl, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, tmpl.UserID, tmpl.WorkspaceID) {
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
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("Insufficient workspace permissions", err)
	}
	scope := getScope(c)
	data := req.Body

	if data.Name == "" {
		return c.AbortBadRequest("template name is required")
	}

	defaultLang := data.DefaultLanguage
	if defaultLang == "" {
		if defLang, err := h.languageRepo.FindDefault(scope); err == nil {
			defaultLang = defLang.Code
		} else {
			defaultLang = "en"
		}
	}

	tmpl := &models.Template{
		UserID:          scope.UserID,
		WorkspaceID:     scope.WorkspaceID,
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

// ImportHTML creates a template from an uploaded .html file.
// It extracts <style> CSS into a separate StyleSheet and uses the HTML body
// as the template localization content.
func (h *TemplateHandler) ImportHTML(c *okapi.Context) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("Insufficient workspace permissions", err)
	}
	scope := getScope(c)

	file, header, err := c.Request().FormFile("file")
	if err != nil {
		return c.AbortBadRequest("file is required")
	}
	defer func() { _ = file.Close() }()

	// Validate file extension
	filename := header.Filename
	lower := strings.ToLower(filename)
	if !strings.HasSuffix(lower, ".html") && !strings.HasSuffix(lower, ".htm") {
		return c.AbortBadRequest("only .html files are accepted")
	}

	// Limit file size to 2MB
	if header.Size > 2*1024*1024 {
		return c.AbortBadRequest("file size must not exceed 2MB")
	}

	// Read file contents
	buf := new(strings.Builder)
	if _, err := io.Copy(buf, file); err != nil {
		return c.AbortBadRequest("failed to read file")
	}
	htmlContent := buf.String()
	if strings.TrimSpace(htmlContent) == "" {
		return c.AbortBadRequest("file is empty")
	}

	// Derive template name from filename
	name := strings.TrimSuffix(lower, ".html")
	name = strings.TrimSuffix(name, ".htm")
	name = strings.TrimSpace(name)
	if name == "" {
		return c.AbortBadRequest("could not derive template name from filename")
	}

	// Extract CSS and HTML body
	css, body := extractStyleAndBody(htmlContent)

	// Try to extract a subject from <title>
	subject := extractTitle(htmlContent)
	if subject == "" {
		subject = name
	}

	// Resolve default language from scope
	defaultLang := "en"
	if defLang, err := h.languageRepo.FindDefault(scope); err == nil {
		defaultLang = defLang.Code
	}

	// Create template
	tmpl := &models.Template{
		UserID:          scope.UserID,
		WorkspaceID:     scope.WorkspaceID,
		Name:            name,
		DefaultLanguage: defaultLang,
		Description:     fmt.Sprintf("Imported from %s", filename),
	}
	if err := h.repo.Create(tmpl); err != nil {
		return c.AbortConflict("template with this name already exists")
	}

	// Create stylesheet from extracted CSS (if any)
	var stylesheetID *uint
	if css != "" {
		ss := &models.StyleSheet{
			UserID:      scope.UserID,
			WorkspaceID: scope.WorkspaceID,
			Name:        name + "-styles",
			CSS:         css,
		}
		if err := h.stylesheetRepo.Create(ss); err == nil {
			stylesheetID = &ss.ID
		}
	}

	// Create version v1
	v := &models.TemplateVersion{
		TemplateID:   tmpl.ID,
		Version:      1,
		StyleSheetID: stylesheetID,
	}
	if err := h.versionRepo.Create(v); err != nil {
		return c.AbortInternalServerError("failed to create template version")
	}

	// Set active version
	tmpl.ActiveVersionID = &v.ID
	if err := h.repo.Update(tmpl); err != nil {
		return c.AbortInternalServerError("failed to activate version")
	}

	// Create default localization
	loc := &models.TemplateLocalization{
		VersionID:       v.ID,
		Language:        defaultLang,
		SubjectTemplate: subject,
		HTMLTemplate:    body,
	}
	if err := h.localizationRepo.Create(loc); err != nil {
		return c.AbortInternalServerError("failed to create localization")
	}

	return created(c, tmpl)
}
