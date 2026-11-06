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
	"fmt"
	"io"
	"path/filepath"
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
		Name            string `json:"name" doc:"The template's name, as typed: \"Welcome Email\" is stored as the identifier welcome-email, which you send with, and \"Welcome Email\" as display_name. It cannot change later."`
		DisplayName     string `json:"display_name" doc:"The template's label. Defaults to name."`
		SampleData      string `json:"sample_data" default:"{}"`
		DefaultLanguage string `json:"default_language"`
		Description     string `json:"description"`
	} `json:"body"`
}
type UpdateTemplateRequest struct {
	ID   int `param:"id"`
	Body struct {
		Name            string  `json:"name" doc:"A template's name cannot change; only accepted when it is the current one."`
		DisplayName     string  `json:"display_name"`
		SampleData      *string `json:"sample_data" default:"{}"`
		DefaultLanguage string  `json:"default_language"`
		Description     *string `json:"description"`
	} `json:"body"`
}
type GetTemplateRequest struct {
	ID int `param:"id"`
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
		if defLang, err := h.languageRepo.FindDefault(scope); err == nil {
			defaultLang = defLang.Code
		} else {
			defaultLang = "en"
		}
	}

	name, displayName, err := resolveNames(req.Body.Name, req.Body.DisplayName, "")
	if err != nil {
		return c.AbortBadRequest(err.Error())
	}
	tmpl := &models.Template{
		UserID:          scope.UserID,
		WorkspaceID:     scope.WorkspaceID,
		Name:            name,
		DisplayName:     displayName,
		DefaultLanguage: defaultLang,
		Description:     req.Body.Description,
		SampleData:      req.Body.SampleData,
		LastEditedByID:  &scope.UserID,
	}

	err = h.repo.CreateWithVersions(tmpl, []repositories.ImportVersion{
		{SampleData: req.Body.SampleData, IsActive: true},
	})
	if repositories.IsDuplicateName(err) {
		return c.AbortConflict("a template named \"" + tmpl.Name + "\" already exists in this workspace")
	}
	if err != nil {
		return c.AbortInternalServerError("failed to create template")
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

	displayName, err := resolveRename(req.Body.Name, req.Body.DisplayName, "", tmpl.Name, "template")
	if err != nil {
		return c.AbortBadRequest(err.Error())
	}
	if displayName != "" {
		tmpl.DisplayName = displayName
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
	editorID := getScope(c).UserID
	tmpl.LastEditedByID = &editorID

	if err := h.repo.Update(tmpl); err != nil {
		return c.AbortInternalServerError("failed to update template")
	}

	// Reload so the response carries the populated CreatedBy / LastEditedBy refs.
	if updated, ferr := h.repo.FindByIDWithActors(tmpl.ID); ferr == nil {
		tmpl = updated
	}

	return ok(c, tmpl)
}

type TemplateListItem struct {
	models.Template
	Languages []string `json:"languages"`
}

func (h *TemplateHandler) List(c *okapi.Context, req *ListTemplatesRequest) error {
	page, size, offset := normalizePageParams(req.Page, req.Size)

	templates, total, err := h.repo.FindByScope(getScope(c), req.Search, size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list templates")
	}

	ids := make([]uint, 0, len(templates))
	for i := range templates {
		ids = append(ids, templates[i].ID)
	}
	// One aggregate for the page rather than a query per row.
	languages, err := h.repo.LanguagesForActiveVersions(ids)
	if err != nil {
		return c.AbortInternalServerError("failed to list templates")
	}

	items := make([]TemplateListItem, 0, len(templates))
	for i := range templates {
		langs := languages[templates[i].ID]
		if langs == nil {
			langs = []string{}
		}
		items = append(items, TemplateListItem{Template: templates[i], Languages: langs})
	}

	return paginated(c, items, total, page, size)
}

func (h *TemplateHandler) Get(c *okapi.Context, req *GetTemplateRequest) error {
	tmpl, err := h.repo.FindByIDWithActors(uint(req.ID))
	if err != nil || !ownsResource(c, tmpl.UserID, tmpl.WorkspaceID) {
		return c.AbortNotFound("template not found")
	}
	return ok(c, tmpl)
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
	// Reserved {{ posta_* }} variables resolve to their own names so previews render without missing-key errors.
	data = email.WithSystemVarNames(data)

	input := &email.RenderInput{
		SubjectTemplate: req.Body.SubjectTemplate,
		HTMLTemplate:    req.Body.HTMLTemplate,
		TextTemplate:    req.Body.TextTemplate,
	}

	if req.Body.StyleSheetID != nil && *req.Body.StyleSheetID > 0 {
		ss, err := h.stylesheetRepo.FindByIDInScope(getScope(c), *req.Body.StyleSheetID)
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
	BuilderJSON     string `json:"builder_json,omitempty"`
}

type ExportVersion struct {
	Version       int                  `json:"version"`
	SampleData    string               `json:"sample_data"`
	IsActive      bool                 `json:"is_active"`
	StyleSheet    *ExportStyleSheet    `json:"stylesheet,omitempty"`
	Localizations []ExportLocalization `json:"localizations"`
}

type TemplateExport struct {
	PostaVersion    string          `json:"posta_version,omitempty"`
	ExportedAt      string          `json:"exported_at,omitempty"`
	Name            string          `json:"name"`
	DisplayName     string          `json:"display_name,omitempty"`
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
				BuilderJSON:     l.BuilderJSON,
			})
		}

		isActive := tmpl.ActiveVersionID != nil && *tmpl.ActiveVersionID == v.ID
		exportVersions = append(exportVersions, ExportVersion{
			Version:       v.Version,
			SampleData:    v.SampleData,
			IsActive:      isActive,
			StyleSheet:    exportVersionStyleSheet(v.StyleSheet, true),
			Localizations: exportLocs,
		})
	}

	export := TemplateExport{
		PostaVersion:    config.Version,
		ExportedAt:      time.Now().UTC().Format(time.RFC3339),
		Name:            tmpl.Name,
		DisplayName:     tmpl.DisplayName,
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

	if data.Name == "" && data.DisplayName == "" {
		return c.AbortBadRequest("template name is required")
	}
	name, displayName := importNames(data.Name, data.DisplayName, "template")

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
		Name:            name,
		DisplayName:     displayName,
		DefaultLanguage: defaultLang,
		Description:     data.Description,
		SampleData:      data.SampleData,
		LastEditedByID:  &scope.UserID,
	}

	versions := make([]repositories.ImportVersion, 0, len(data.Versions))
	for _, ev := range data.Versions {
		locs := make([]repositories.ImportLocalization, 0, len(ev.Localizations))
		for _, el := range ev.Localizations {
			locs = append(locs, repositories.ImportLocalization{
				Language:        el.Language,
				SubjectTemplate: el.SubjectTemplate,
				HTMLTemplate:    el.HTMLTemplate,
				TextTemplate:    el.TextTemplate,
				BuilderJSON:     el.BuilderJSON,
			})
		}
		versions = append(versions, repositories.ImportVersion{
			SampleData:    ev.SampleData,
			StyleSheetID:  findOrCreateStyleSheet(ev.StyleSheet, scope, h.stylesheetRepo),
			IsActive:      ev.IsActive,
			Localizations: locs,
		})
	}
	if len(versions) == 0 {
		versions = append(versions, repositories.ImportVersion{SampleData: data.SampleData, IsActive: true})
	}

	err := h.repo.CreateWithVersions(tmpl, versions)
	if repositories.IsDuplicateName(err) {
		return c.AbortConflict("a template named \"" + tmpl.Name + "\" already exists in this workspace")
	}
	if err != nil {
		return c.AbortInternalServerError("failed to import template")
	}

	return created(c, tmpl)
}

// ImportHTML moves <style> CSS into a separate StyleSheet and uses the body as the localization content.
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

	filename := header.Filename
	lower := strings.ToLower(filename)
	if !strings.HasSuffix(lower, ".html") && !strings.HasSuffix(lower, ".htm") {
		return c.AbortBadRequest("only .html files are accepted")
	}

	if header.Size > 2*1024*1024 {
		return c.AbortBadRequest("file size must not exceed 2MB")
	}

	buf := new(strings.Builder)
	if _, err := io.Copy(buf, file); err != nil {
		return c.AbortBadRequest("failed to read file")
	}
	htmlContent := buf.String()
	if strings.TrimSpace(htmlContent) == "" {
		return c.AbortBadRequest("file is empty")
	}

	// Optional form fields override what is otherwise derived from the filename and the document.
	form := func(key string) string { return strings.TrimSpace(c.Request().FormValue(key)) }

	// Named after the filename, keeping its capitalisation; the lowercased copy only matches the extension.
	name := form("display_name")
	if name == "" && form("name") == "" {
		name = strings.TrimSpace(filename[:len(filename)-len(filepath.Ext(filename))])
		if name == "" || models.ToIdentifier(name) == "" {
			return c.AbortBadRequest("could not derive template name from filename")
		}
	}
	identifier, displayName, err := resolveNames(form("name"), name, "")
	if err != nil {
		return c.AbortBadRequest(err.Error())
	}
	name = displayName

	css, body := extractStyleAndBody(htmlContent)
	linkNames := extractStyleSheetLinks(htmlContent)

	subject := form("subject")
	if subject == "" {
		subject = extractTitle(htmlContent)
	}
	if subject == "" {
		subject = name
	}

	defaultLang := form("language")
	if defaultLang == "" {
		defaultLang = "en"
		if defLang, err := h.languageRepo.FindDefault(scope); err == nil {
			defaultLang = defLang.Code
		}
	}
	description := form("description")
	if description == "" {
		description = fmt.Sprintf("Imported from %s", filename)
	}

	stylesheetID := h.resolveImportedStyleSheet(scope, name, css, linkNames)

	tmpl := &models.Template{
		UserID:          scope.UserID,
		WorkspaceID:     scope.WorkspaceID,
		Name:            identifier,
		DisplayName:     displayName,
		DefaultLanguage: defaultLang,
		Description:     description,
		SampleData:      "{}",
		LastEditedByID:  &scope.UserID,
	}

	err = h.repo.CreateWithVersions(tmpl, []repositories.ImportVersion{{
		SampleData:   "{}",
		StyleSheetID: stylesheetID,
		IsActive:     true,
		Localizations: []repositories.ImportLocalization{{
			Language:        defaultLang,
			SubjectTemplate: subject,
			HTMLTemplate:    body,
		}},
	}})
	if repositories.IsDuplicateName(err) {
		return c.AbortConflict("a template named \"" + tmpl.Name + "\" already exists in this workspace")
	}
	if err != nil {
		return c.AbortInternalServerError("failed to import template")
	}

	return created(c, tmpl)
}

func (h *TemplateHandler) resolveImportedStyleSheet(scope repositories.ResourceScope, baseName, inlineCSS string, linkNames []string) *uint {
	for _, name := range linkNames {
		if name == "" {
			continue
		}
		if ss, err := h.stylesheetRepo.FindByNameInScope(scope, name); err == nil && ss != nil {
			return &ss.ID
		}
	}

	for _, name := range linkNames {
		if name != "" {
			return h.createStyleSheet(scope, name, inlineCSS)
		}
	}

	if strings.TrimSpace(inlineCSS) != "" {
		return findOrCreateStyleSheet(&ExportStyleSheet{Name: models.ToIdentifier(baseName) + "-styles", DisplayName: baseName + " styles", CSS: inlineCSS}, scope, h.stylesheetRepo)
	}

	return nil
}

func (h *TemplateHandler) createStyleSheet(scope repositories.ResourceScope, name, css string) *uint {
	id, displayName := importNames(name, "", "stylesheet")
	ss := &models.StyleSheet{
		UserID:      scope.UserID,
		WorkspaceID: scope.WorkspaceID,
		Name:        id,
		DisplayName: displayName,
		CSS:         css,
	}
	if err := h.stylesheetRepo.Create(ss); err != nil {
		return nil
	}
	return &ss.ID
}
