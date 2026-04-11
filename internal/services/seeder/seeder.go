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

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/storage/repositories"
	goutils "github.com/jkaninda/go-utils"
	"github.com/jkaninda/logger"
	"github.com/jkaninda/okapi"
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

type templateDef struct {
	Name        string
	Description string
	SampleData  okapi.M
	SubjectEN   string
	HTMLEN      string
	TextEN      string
	SubjectFR   string
	HTMLFR      string
	TextFR      string
}

// seedTemplate creates a single template with version and localizations.
func (s *Seeder) seedTemplate(userID uint, ssID uint, def templateDef) {
	b, _ := json.MarshalIndent(def.SampleData, "", "  ")
	sampleData := string(b)

	tmpl := &models.Template{
		UserID:          userID,
		Name:            def.Name,
		DefaultLanguage: "en",
		Description:     def.Description,
		SampleData:      sampleData,
	}
	if err := s.templateRepo.Create(tmpl); err != nil {
		logger.Error("failed to seed template", "name", def.Name, "user_id", userID, "error", err)
		return
	}

	v := &models.TemplateVersion{
		TemplateID:   tmpl.ID,
		Version:      1,
		StyleSheetID: &ssID,
		SampleData:   sampleData,
	}
	if err := s.versionRepo.Create(v); err != nil {
		logger.Error("failed to seed template version", "name", def.Name, "user_id", userID, "error", err)
		return
	}

	enLoc := &models.TemplateLocalization{
		VersionID:       v.ID,
		Language:        "en",
		SubjectTemplate: def.SubjectEN,
		HTMLTemplate:    def.HTMLEN,
		TextTemplate:    def.TextEN,
	}
	if err := s.localizationRepo.Create(enLoc); err != nil {
		logger.Error("failed to seed English localization", "name", def.Name, "user_id", userID, "error", err)
		return
	}

	frLoc := &models.TemplateLocalization{
		VersionID:       v.ID,
		Language:        "fr",
		SubjectTemplate: def.SubjectFR,
		HTMLTemplate:    def.HTMLFR,
		TextTemplate:    def.TextFR,
	}
	if err := s.localizationRepo.Create(frLoc); err != nil {
		logger.Error("failed to seed French localization", "name", def.Name, "user_id", userID, "error", err)
	}

	vID := v.ID
	tmpl.ActiveVersionID = &vID
	if err := s.templateRepo.Update(tmpl); err != nil {
		logger.Error("failed to activate template version", "name", def.Name, "user_id", userID, "error", err)
	}
}

// SeedUserDefaults creates default stylesheet and templates for a user
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
		Name:   "default",
		CSS:    defaultCSS,
	}
	if err := s.stylesheetRepo.Create(ss); err != nil {
		logger.Error("failed to seed default stylesheet", "user_id", userID, "error", err)
		return
	}

	year := time.Now().Year()
	docsURL := fmt.Sprintf("%s/docs", goutils.Env("POSTA_WEB_URL", ""))

	// 1. Welcome template
	s.seedTemplate(userID, ss.ID, templateDef{
		Name:        "Welcome Email",
		Description: "Welcome email introducing Posta and its features",
		SampleData: okapi.M{
			"name":    userName,
			"product": "Posta",
			"company": "Posta",
			"year":    year,
			"docs":    docsURL,
			"features": []string{
				"REST API for transactional, batch, and templated emails",
				"Scheduled sending and preview mode",
				"Async processing with automatic retries and priority queues",
				"Versioned and multi-language templates with variable substitution",
				"Multiple SMTP providers with TLS and shared pools",
				"Domain verification (SPF, DKIM, DMARC)",
				"API keys with expiration, hashing, and IP allowlisting",
				"JWT authentication, RBAC, and two-factor authentication",
				"Contact tracking, segmentation, and suppression lists",
				"Multi-tenant workspaces with scoped API keys",
				"Event-driven webhooks with retry and delivery tracking",
				"Email delivery analytics and Prometheus metrics",
				"Web dashboard with dark/light mode",
				"Official SDKs for Go, PHP, and Java",
			},
			"links": []map[string]string{
				{"title": "Website", "url": "https://goposta.dev/"},
				{"title": "API Documentation", "url": "https://app.goposta.dev/docs"},
				{"title": "Documentation", "url": "https://docs.goposta.dev/"},
				{"title": "GitHub Repository", "url": "https://github.com/goposta/posta"},
			},
		},
		SubjectEN: "Welcome to Posta, {{name}}!",
		HTMLEN:    defaultHTMLTemplate,
		TextEN:    defaultTextTemplate,
		SubjectFR: "Bienvenue sur Posta, {{name}} !",
		HTMLFR:    defaultHTMLTemplateFr,
		TextFR:    defaultTextTemplateFr,
	})

	// 2. Password Reset template
	s.seedTemplate(userID, ss.ID, templateDef{
		Name:        "Password Reset",
		Description: "Transactional email for password reset requests",
		SampleData: okapi.M{
			"name":      userName,
			"company":   "Posta",
			"year":      year,
			"resetLink": "https://example.com/reset?token=abc123",
			"expiry":    "1 hour",
		},
		SubjectEN: "Reset your password, {{name}}",
		HTMLEN:    passwordResetHTMLTemplate,
		TextEN:    passwordResetTextTemplate,
		SubjectFR: "Réinitialisez votre mot de passe, {{name}}",
		HTMLFR:    passwordResetHTMLTemplateFr,
		TextFR:    passwordResetTextTemplateFr,
	})

	// 3. Order Confirmation template
	s.seedTemplate(userID, ss.ID, templateDef{
		Name:        "Order Confirmation",
		Description: "Order confirmation email with item details and total",
		SampleData: okapi.M{
			"name":        userName,
			"company":     "Posta",
			"year":        year,
			"orderNumber": "10042",
			"total":       "$129.97",
			"items": []map[string]interface{}{
				{"name": "Wireless Keyboard", "qty": 1, "price": "$59.99"},
				{"name": "USB-C Hub", "qty": 2, "price": "$34.99"},
			},
		},
		SubjectEN: "Order #{{orderNumber}} confirmed",
		HTMLEN:    orderConfirmationHTMLTemplate,
		TextEN:    orderConfirmationTextTemplate,
		SubjectFR: "Commande #{{orderNumber}} confirmée",
		HTMLFR:    orderConfirmationHTMLTemplateFr,
		TextFR:    orderConfirmationTextTemplateFr,
	})

	// 4. Newsletter template
	s.seedTemplate(userID, ss.ID, templateDef{
		Name:        "Monthly Newsletter",
		Description: "Monthly newsletter with articles and unsubscribe link",
		SampleData: okapi.M{
			"name":    userName,
			"company": "Posta",
			"year":    year,
			"month":   "April",
			"articles": []map[string]string{
				{
					"title":   "Introducing Webhooks",
					"summary": "Track every email event in real time with our new webhook system. Configure endpoints, set retry policies, and monitor delivery.",
					"url":     "https://docs.goposta.dev/webhooks",
				},
				{
					"title":   "Template Versioning Guide",
					"summary": "Learn how to manage template versions, roll back changes, and preview before publishing.",
					"url":     "https://docs.goposta.dev/templates/versioning",
				},
				{
					"title":   "Multi-language Emails",
					"summary": "Send localized emails to your global audience with built-in language support and fallback chains.",
					"url":     "https://docs.goposta.dev/templates/localization",
				},
			},
			"unsubscribeUrl": "https://example.com/unsubscribe?token=xyz",
		},
		SubjectEN: "{{company}} — {{month}} Newsletter",
		HTMLEN:    newsletterHTMLTemplate,
		TextEN:    newsletterTextTemplate,
		SubjectFR: "{{company}} — Newsletter de {{month}}",
		HTMLFR:    newsletterHTMLTemplateFr,
		TextFR:    newsletterTextTemplateFr,
	})

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

	logger.Info("seeded default stylesheet, templates, versions, localizations, and languages", "user_id", userID)
}
