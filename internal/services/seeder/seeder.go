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

// seedLanguages is the ordered set of languages every seeded template ships in.
// The first entry is used as the template's default language. Each code must
// have a matching templates/<code>/ folder and a Subjects entry per def.
var seedLanguages = []string{"en", "fr", "de"}

type templateDef struct {
	Name        string
	Description string
	Base        string // template file base name, e.g. "welcome"
	SampleData  okapi.M
	Subjects    map[string]string // language code -> subject line
}

// seedTemplate creates a single template with a version and one localization
// per seedLanguages entry, loading each localized body from the embedded files.
func (s *Seeder) seedTemplate(workspaceID, userID uint, ssID uint, def templateDef) {
	b, _ := json.MarshalIndent(def.SampleData, "", "  ")
	sampleData := string(b)

	tmpl := &models.Template{
		UserID:          userID,
		WorkspaceID:     workspacePtr(workspaceID),
		Name:            def.Name,
		DefaultLanguage: seedLanguages[0],
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

	for _, lang := range seedLanguages {
		loc := &models.TemplateLocalization{
			VersionID:       v.ID,
			Language:        lang,
			SubjectTemplate: def.Subjects[lang],
			HTMLTemplate:    htmlTmpl(def.Base, lang),
			TextTemplate:    textTmpl(def.Base, lang),
		}
		if err := s.localizationRepo.Create(loc); err != nil {
			logger.Error("failed to seed localization", "name", def.Name, "lang", lang, "user_id", userID, "error", err)
		}
	}

	vID := v.ID
	tmpl.ActiveVersionID = &vID
	if err := s.templateRepo.Update(tmpl); err != nil {
		logger.Error("failed to activate template version", "name", def.Name, "user_id", userID, "error", err)
	}
}

// workspacePtr returns a pointer to workspaceID, or nil when zero (defensive:
// seeding always runs against a real personal workspace post-migration).
func workspacePtr(workspaceID uint) *uint {
	if workspaceID == 0 {
		return nil
	}
	return &workspaceID
}

func (s *Seeder) SeedWorkspaceDefaults(workspaceID, userID uint, userName string) {
	if userName == "" {
		userName = "Jonas"
	}
	templates, total, err := s.templateRepo.FindByUserID(userID, 1, 0)
	if err != nil || total > 0 || len(templates) > 0 {
		return
	}

	wsID := workspacePtr(workspaceID)

	// Create default stylesheet
	ss := &models.StyleSheet{
		UserID:      userID,
		WorkspaceID: wsID,
		Name:        "default",
		CSS:         defaultCSS,
	}
	if err := s.stylesheetRepo.Create(ss); err != nil {
		logger.Error("failed to seed default stylesheet", "user_id", userID, "error", err)
		return
	}

	year := time.Now().Year()
	docsURL := fmt.Sprintf("%s/docs", goutils.Env("POSTA_WEB_URL", ""))

	for _, def := range defaultTemplateDefs(userName, year, docsURL) {
		s.seedTemplate(workspaceID, userID, ss.ID, def)
	}

	// Seed default languages
	defaultLanguages := []struct {
		Code string
		Name string
	}{
		{"en", "English"},
		{"fr", "French"},
		{"de", "German"},
	}
	for _, dl := range defaultLanguages {
		lang := &models.Language{UserID: userID, WorkspaceID: wsID, Code: dl.Code, Name: dl.Name}
		if lang.Code == "en" {
			lang.IsDefault = true
		}
		if err := s.languageRepo.Create(lang); err != nil {
			logger.Error("failed to seed language", "user_id", userID, "code", dl.Code, "error", err)
		}
	}

	logger.Info("seeded default stylesheet, templates, versions, localizations, and languages",
		"user_id", userID, "workspace_id", workspaceID)
}

// defaultTemplateDefs returns the built-in templates seeded for every new user.
// Kept as a pure function (no DB) so it can be rendered in tests to verify each
// template parses and every referenced variable is covered by its sample data.
func defaultTemplateDefs(userName string, year int, docsURL string) []templateDef {
	return []templateDef{
		// 1. Welcome
		{
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
			Base: "welcome",
			Subjects: map[string]string{
				"en": "Welcome to Posta, {{name}}!",
				"fr": "Bienvenue sur Posta, {{name}} !",
				"de": "Willkommen bei Posta, {{name}}!",
			},
		},
		// 2. Password Reset
		{
			Name:        "Password Reset",
			Description: "Transactional email for password reset requests",
			SampleData: okapi.M{
				"name":      userName,
				"company":   "Posta",
				"year":      year,
				"resetLink": "https://example.com/reset?token=abc123",
				"expiry":    "1 hour",
			},
			Base: "password-reset",
			Subjects: map[string]string{
				"en": "Reset your password, {{name}}",
				"fr": "Réinitialisez votre mot de passe, {{name}}",
				"de": "Setzen Sie Ihr Passwort zurück, {{name}}",
			},
		},
		// 3. Order Confirmation
		{
			Name:        "Order Confirmation",
			Description: "Order confirmation email with item details and total",
			SampleData: okapi.M{
				"name":        userName,
				"company":     "Posta",
				"year":        year,
				"orderNumber": "10042",
				"orderDate":   "April 21, 2026",
				"total":       "$129.97",
				"items": []map[string]interface{}{
					{"name": "Wireless Keyboard", "qty": 1, "price": "$59.99"},
					{"name": "USB-C Hub", "qty": 2, "price": "$34.99"},
				},
			},
			Base: "order-confirmation",
			Subjects: map[string]string{
				"en": "Order #{{orderNumber}} confirmed",
				"fr": "Commande #{{orderNumber}} confirmée",
				"de": "Bestellung #{{orderNumber}} bestätigt",
			},
		},
		// 4. Newsletter
		{
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
			Base: "newsletter",
			Subjects: map[string]string{
				"en": "{{company}} — {{month}} Newsletter",
				"fr": "{{company}} — Newsletter de {{month}}",
				"de": "{{company}} — Newsletter {{month}}",
			},
		},
		// 5. Email Verification
		{
			Name:        "Email Verification",
			Description: "Confirm a new account's email address with a link and code",
			SampleData: okapi.M{
				"name":       userName,
				"company":    "Posta",
				"year":       year,
				"verifyLink": "https://example.com/verify?token=abc123",
				"code":       "492018",
				"expiry":     "24 hours",
			},
			Base: "verify-email",
			Subjects: map[string]string{
				"en": "Verify your email address",
				"fr": "Vérifiez votre adresse e-mail",
				"de": "Bestätigen Sie Ihre E-Mail-Adresse",
			},
		},
		// 6. Sign-in Code (OTP)
		{
			Name:        "Sign-in Code",
			Description: "One-time passcode / magic-link login email",
			SampleData: okapi.M{
				"name":      userName,
				"company":   "Posta",
				"year":      year,
				"code":      "731924",
				"loginLink": "https://example.com/login?token=abc123",
				"expiry":    "10 minutes",
			},
			Base: "sign-in-code",
			Subjects: map[string]string{
				"en": "Your {{company}} sign-in code",
				"fr": "Votre code de connexion {{company}}",
				"de": "Ihr {{company}}-Anmeldecode",
			},
		},
		// 7. Invoice / Receipt
		{
			Name:        "Invoice Receipt",
			Description: "Payment receipt with line items, subtotal, tax, and total",
			SampleData: okapi.M{
				"name":          userName,
				"company":       "Posta",
				"year":          year,
				"invoiceNumber": "INV-2042",
				"invoiceDate":   "April 21, 2026",
				"subtotal":      "$118.00",
				"tax":           "$11.97",
				"total":         "$129.97",
				"invoiceUrl":    "https://example.com/invoices/INV-2042.pdf",
				"items": []map[string]interface{}{
					{"name": "Pro plan (monthly)", "qty": 1, "price": "$99.00"},
					{"name": "Additional sending domain", "qty": 1, "price": "$19.00"},
				},
			},
			Base: "invoice",
			Subjects: map[string]string{
				"en": "Receipt for invoice {{invoiceNumber}}",
				"fr": "Reçu pour la facture {{invoiceNumber}}",
				"de": "Beleg für Rechnung {{invoiceNumber}}",
			},
		},
		// 8. Team Invitation
		{
			Name:        "Team Invitation",
			Description: "Invite a user to a workspace with an accept link and role",
			SampleData: okapi.M{
				"name":          userName,
				"company":       "Posta",
				"year":          year,
				"inviterName":   "Jonas",
				"workspaceName": "Acme Marketing",
				"role":          "Editor",
				"acceptLink":    "https://example.com/invite?token=abc123",
				"expiry":        "7 days",
			},
			Base: "team-invite",
			Subjects: map[string]string{
				"en": "{{inviterName}} invited you to join {{workspaceName}}",
				"fr": "{{inviterName}} vous invite à rejoindre {{workspaceName}}",
				"de": "{{inviterName}} hat Sie zu {{workspaceName}} eingeladen",
			},
		},
	}
}
