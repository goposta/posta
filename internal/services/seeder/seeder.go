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

package seeder

import (
	"encoding/json"
	"fmt"
	"strings"
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
	formRepo         *repositories.FormRepository
}

func New(
	templateRepo *repositories.TemplateRepository,
	stylesheetRepo *repositories.StyleSheetRepository,
	versionRepo *repositories.TemplateVersionRepository,
	localizationRepo *repositories.TemplateLocalizationRepository,
	languageRepo *repositories.LanguageRepository,
	formRepo *repositories.FormRepository,
) *Seeder {
	return &Seeder{
		templateRepo:     templateRepo,
		stylesheetRepo:   stylesheetRepo,
		versionRepo:      versionRepo,
		localizationRepo: localizationRepo,
		languageRepo:     languageRepo,
		formRepo:         formRepo,
	}
}

// seedLanguages: the first is the default; each needs a templates/<code>/ folder and a Subjects entry per def.
var seedLanguages = []string{"en", "fr", "de"}

type templateDef struct {
	DisplayName string
	Description string
	Base        string // template file base name, e.g. "welcome"
	SampleData  okapi.M
	Subjects    map[string]string // language code -> subject line
}

// sampleBrand fills {{ company }} and {{ product }} in the seeded templates' preview data.
const sampleBrand = "Posta"

func (s *Seeder) seedTemplate(workspaceID, userID uint, ssID uint, def templateDef) {
	b, _ := json.MarshalIndent(def.SampleData, "", "  ")
	sampleData := string(b)

	tmpl := &models.Template{
		UserID:          userID,
		WorkspaceID:     workspacePtr(workspaceID),
		Name:            models.ToIdentifier(def.DisplayName),
		DisplayName:     def.DisplayName,
		DefaultLanguage: seedLanguages[0],
		Description:     def.Description,
		SampleData:      sampleData,
	}
	if err := s.templateRepo.Create(tmpl); err != nil {
		logger.Error("failed to seed template", "name", def.DisplayName, "user_id", userID, "error", err)
		return
	}

	v := &models.TemplateVersion{
		TemplateID:   tmpl.ID,
		Version:      1,
		StyleSheetID: &ssID,
		SampleData:   sampleData,
	}
	if err := s.versionRepo.Create(v); err != nil {
		logger.Error("failed to seed template version", "name", def.DisplayName, "user_id", userID, "error", err)
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
			logger.Error("failed to seed localization", "name", def.DisplayName, "lang", lang, "user_id", userID, "error", err)
		}
	}

	vID := v.ID
	tmpl.ActiveVersionID = &vID
	if err := s.templateRepo.Update(tmpl); err != nil {
		logger.Error("failed to activate template version", "name", def.DisplayName, "user_id", userID, "error", err)
	}
}

// workspacePtr returns nil for a zero ID (defensive: seeding always runs against a real workspace).
func workspacePtr(workspaceID uint) *uint {
	if workspaceID == 0 {
		return nil
	}
	return &workspaceID
}

// sampleRecipientName greets generically when the owner's name is unknown, rather than naming a stranger.
const sampleRecipientName = "there"

func (s *Seeder) SeedWorkspaceDefaults(workspaceID, userID uint, userName string) {
	userName = strings.TrimSpace(userName)
	if userName == "" {
		userName = sampleRecipientName
	}
	// Scoped to the workspace, not the user: each workspace needs its own defaults and re-runs must not duplicate.
	scope := repositories.ResourceScope{UserID: userID, WorkspaceID: &workspaceID}
	templates, total, err := s.templateRepo.FindByScope(scope, "", 1, 0)
	if err != nil || total > 0 || len(templates) > 0 {
		return
	}

	wsID := workspacePtr(workspaceID)

	ss := &models.StyleSheet{
		UserID:      userID,
		WorkspaceID: wsID,
		Name:        "default",
		DisplayName: "Default",
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

	s.seedForms(workspaceID, userID)

	logger.Info("seeded default stylesheet, templates, versions, localizations, languages, and forms",
		"user_id", userID, "workspace_id", workspaceID)
}

// defaultTemplateDefs is pure (no DB) so tests can render each template against its sample data.
func defaultTemplateDefs(userName string, year int, docsURL string) []templateDef {
	return []templateDef{
		{
			DisplayName: "Welcome Email",
			Description: "Welcome email introducing Posta and its features",
			SampleData: okapi.M{
				"name":    userName,
				"product": sampleBrand,
				"company": sampleBrand,
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
		{
			DisplayName: "Password Reset",
			Description: "Transactional email for password reset requests",
			SampleData: okapi.M{
				"name":      userName,
				"company":   sampleBrand,
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
		{
			DisplayName: "Order Confirmation",
			Description: "Order confirmation email with item details and total",
			SampleData: okapi.M{
				"name":        userName,
				"company":     sampleBrand,
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
		{
			DisplayName: "Monthly Newsletter",
			Description: "Monthly newsletter with articles and unsubscribe link",
			SampleData: okapi.M{
				"name":    userName,
				"company": sampleBrand,
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
		{
			DisplayName: "Email Verification",
			Description: "Confirm a new account's email address with a link and code",
			SampleData: okapi.M{
				"name":       userName,
				"company":    sampleBrand,
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
		{
			DisplayName: "Sign-in Code",
			Description: "One-time passcode / magic-link login email",
			SampleData: okapi.M{
				"name":      userName,
				"company":   sampleBrand,
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
		{
			DisplayName: "Invoice Receipt",
			Description: "Payment receipt with line items, subtotal, tax, and total",
			SampleData: okapi.M{
				"name":          userName,
				"company":       sampleBrand,
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
		{
			DisplayName: "Team Invitation",
			Description: "Invite a user to a workspace with an accept link and role",
			SampleData: okapi.M{
				"name":          userName,
				"company":       sampleBrand,
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
