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

package seeder

import (
	"encoding/json"
	"fmt"
	"time"

	goutils "github.com/jkaninda/go-utils"
	"github.com/jkaninda/logger"
	"github.com/jkaninda/okapi"
	"github.com/jkaninda/posta/internal/models"
	"github.com/jkaninda/posta/internal/storage/repositories"
)

type Seeder struct {
	templateRepo     *repositories.TemplateRepository
	stylesheetRepo   *repositories.StyleSheetRepository
	versionRepo      *repositories.TemplateVersionRepository
	localizationRepo *repositories.TemplateLocalizationRepository
	languageRepo     *repositories.LanguageRepository
}

func New(
	templateRepo *repositories.TemplateRepository,
	stylesheetRepo *repositories.StyleSheetRepository,
	versionRepo *repositories.TemplateVersionRepository,
	localizationRepo *repositories.TemplateLocalizationRepository,
	languageRepo *repositories.LanguageRepository,
) *Seeder {
	return &Seeder{
		templateRepo:     templateRepo,
		stylesheetRepo:   stylesheetRepo,
		versionRepo:      versionRepo,
		localizationRepo: localizationRepo,
		languageRepo:     languageRepo,
	}
}

// SeedUserDefaults creates default stylesheet and template for a user
func (s *Seeder) SeedUserDefaults(userID uint, userName string) {
	if userName == "" {
		userName = "Jonas"
	}
	templates, total, err := s.templateRepo.FindByUserID(userID, 1, 0)
	if err != nil || total > 0 || len(templates) > 0 {
		return
	}

	// Create default stylesheet
	ss := &models.StyleSheet{
		UserID: userID,
		Name:   "Default",
		CSS:    defaultCSS,
	}
	if err := s.stylesheetRepo.Create(ss); err != nil {
		logger.Error("failed to seed default stylesheet", "user_id", userID, "error", err)
		return
	}

	sample := okapi.M{
		"name":    userName,
		"product": "Posta",
		"company": "Posta",
		"year":    time.Now().Year(),
		"docs":    fmt.Sprintf("%s/docs", goutils.Env("POSTA_WEB_URL", "")),
		"features": []string{
			"REST Email API",
			"Versioned templates with localization",
			"Multiple SMTP server management",
			"Domain verification (SPF, DKIM, DMARC)",
			"Email analytics and event webhooks",
		},

		"links": []map[string]string{
			{
				"title": "API Documentation",
				"url":   "/docs",
			},
			{
				"title": "GitHub Repository",
				"url":   "https://github.com/jkaninda/posta",
			},
		},
	}

	b, _ := json.MarshalIndent(sample, "", "  ")
	sampleData := string(b)

	// Create default template linked to the stylesheet
	tmpl := &models.Template{
		UserID:          userID,
		Name:            "Welcome Email",
		DefaultLanguage: "en",
		Description:     "Welcome email introducing Posta and its features",
		SampleData:      sampleData,
	}
	if err := s.templateRepo.Create(tmpl); err != nil {
		logger.Error("failed to seed default template", "user_id", userID, "error", err)
		return
	}

	// Create a default version with an English localization
	v := &models.TemplateVersion{
		TemplateID:   tmpl.ID,
		Version:      1,
		StyleSheetID: &ss.ID,
		SampleData:   sampleData,
	}
	if err := s.versionRepo.Create(v); err != nil {
		logger.Error("failed to seed default template version", "user_id", userID, "error", err)
		return
	}

	l := &models.TemplateLocalization{
		VersionID:       v.ID,
		Language:        "en",
		SubjectTemplate: "Welcome to Posta, {{name}}!",
		HTMLTemplate:    defaultHTMLTemplate,
		TextTemplate:    defaultTextTemplate,
	}
	if err := s.localizationRepo.Create(l); err != nil {
		logger.Error("failed to seed default localization", "user_id", userID, "error", err)
		return
	}

	lFr := &models.TemplateLocalization{
		VersionID:       v.ID,
		Language:        "fr",
		SubjectTemplate: "Bienvenue sur Posta, {{name}} !",
		HTMLTemplate:    defaultHTMLTemplateFr,
		TextTemplate:    defaultTextTemplateFr,
	}
	if err := s.localizationRepo.Create(lFr); err != nil {
		logger.Error("failed to seed French localization", "user_id", userID, "error", err)
	}

	// Activate the version
	vID := v.ID
	tmpl.ActiveVersionID = &vID
	if err := s.templateRepo.Update(tmpl); err != nil {
		logger.Error("failed to activate default version", "user_id", userID, "error", err)
		return
	}

	// Seed default languages
	defaultLanguages := []struct {
		Code string
		Name string
	}{
		{"en", "English"},
		{"fr", "French"},
	}
	for _, dl := range defaultLanguages {
		lang := &models.Language{UserID: userID, Code: dl.Code, Name: dl.Name}
		if err := s.languageRepo.Create(lang); err != nil {
			logger.Error("failed to seed language", "user_id", userID, "code", dl.Code, "error", err)
		}
	}

	logger.Info("seeded default stylesheet, template, version, localization, and languages", "user_id", userID)
}
