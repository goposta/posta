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

	"github.com/goposta/posta/internal/config"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
	"gorm.io/gorm"
)

type WorkspaceDataHandler struct {
	db                 *gorm.DB
	workspaceRepo      *repositories.WorkspaceRepository
	templateRepo       *repositories.TemplateRepository
	versionRepo        *repositories.TemplateVersionRepository
	localizationRepo   *repositories.TemplateLocalizationRepository
	stylesheetRepo     *repositories.StyleSheetRepository
	languageRepo       *repositories.LanguageRepository
	contactRepo        *repositories.ContactRepository
	contactListRepo    *repositories.ContactListRepository
	webhookRepo        *repositories.WebhookRepository
	suppressionRepo    *repositories.SuppressionRepository
	smtpRepo           *repositories.SMTPRepository
	domainRepo         *repositories.DomainRepository
	subscriberRepo     *repositories.SubscriberRepository
	subscriberListRepo *repositories.SubscriberListRepository
}

func NewWorkspaceDataHandler(
	db *gorm.DB,
	workspaceRepo *repositories.WorkspaceRepository,
	templateRepo *repositories.TemplateRepository,
	versionRepo *repositories.TemplateVersionRepository,
	localizationRepo *repositories.TemplateLocalizationRepository,
	stylesheetRepo *repositories.StyleSheetRepository,
	languageRepo *repositories.LanguageRepository,
	contactRepo *repositories.ContactRepository,
	contactListRepo *repositories.ContactListRepository,
	webhookRepo *repositories.WebhookRepository,
	suppressionRepo *repositories.SuppressionRepository,
	smtpRepo *repositories.SMTPRepository,
	domainRepo *repositories.DomainRepository,
	subscriberRepo *repositories.SubscriberRepository,
	subscriberListRepo *repositories.SubscriberListRepository,
) *WorkspaceDataHandler {
	return &WorkspaceDataHandler{
		db:                 db,
		workspaceRepo:      workspaceRepo,
		templateRepo:       templateRepo,
		versionRepo:        versionRepo,
		localizationRepo:   localizationRepo,
		stylesheetRepo:     stylesheetRepo,
		languageRepo:       languageRepo,
		contactRepo:        contactRepo,
		contactListRepo:    contactListRepo,
		webhookRepo:        webhookRepo,
		suppressionRepo:    suppressionRepo,
		smtpRepo:           smtpRepo,
		domainRepo:         domainRepo,
		subscriberRepo:     subscriberRepo,
		subscriberListRepo: subscriberListRepo,
	}
}

type WorkspaceDataExport struct {
	PostaVersion      string                  `json:"posta_version"`
	ExportedAt        string                  `json:"exported_at"`
	WorkspaceSettings ExportWorkspaceSettings `json:"workspace_settings"`
	Templates         []TemplateExport        `json:"templates"`
	Stylesheets       []ExportStyleSheet      `json:"stylesheets"`
	Languages         []ExportLanguage        `json:"languages"`
	Contacts          []ExportContact         `json:"contacts"`
	ContactLists      []ExportContactList     `json:"contact_lists"`
	Suppressions      []ExportSuppression     `json:"suppressions"`
	Webhooks          []ExportWebhook         `json:"webhooks"`
	SMTPServers       []ExportSMTPServer      `json:"smtp_servers"`
	Domains           []ExportDomain          `json:"domains"`
	Subscribers       []ExportSubscriber      `json:"subscribers"`
	SubscriberLists   []ExportSubscriberList  `json:"subscriber_lists"`
}

type ImportWorkspaceDataRequest struct {
	Body WorkspaceDataExport `json:"body"`
}

func (h *WorkspaceDataHandler) Export(c *okapi.Context) error {
	wsID := uint(c.GetInt("workspace_id"))
	scope := repositories.ResourceScope{WorkspaceID: &wsID}

	ws, err := h.workspaceRepo.FindByID(wsID)
	if err != nil {
		return c.AbortNotFound("workspace not found")
	}

	templates, _, err := h.templateRepo.FindByScope(scope, "", exportLimit, 0)
	if err != nil {
		return c.AbortInternalServerError("failed to load templates")
	}
	tmplExport, err := exportTemplates(templates, h.versionRepo, h.localizationRepo)
	if err != nil {
		return c.AbortInternalServerError("failed to load template details")
	}

	stylesheets, _, err := h.stylesheetRepo.FindByScope(scope, exportLimit, 0)
	if err != nil {
		return c.AbortInternalServerError("failed to load stylesheets")
	}

	languages, _, err := h.languageRepo.FindByScope(scope, exportLimit, 0)
	if err != nil {
		return c.AbortInternalServerError("failed to load languages")
	}

	contacts, _, err := h.contactRepo.FindByScope(scope, "", exportLimit, 0)
	if err != nil {
		return c.AbortInternalServerError("failed to load contacts")
	}

	contactLists, _, err := h.contactListRepo.FindByScope(scope, exportLimit, 0)
	if err != nil {
		return c.AbortInternalServerError("failed to load contact lists")
	}

	suppressions, _, err := h.suppressionRepo.FindByScope(scope, exportLimit, 0)
	if err != nil {
		return c.AbortInternalServerError("failed to load suppressions")
	}

	webhooks, _, err := h.webhookRepo.FindByScope(scope, exportLimit, 0)
	if err != nil {
		return c.AbortInternalServerError("failed to load webhooks")
	}

	smtpServers, _, err := h.smtpRepo.FindByScope(scope, exportLimit, 0)
	if err != nil {
		return c.AbortInternalServerError("failed to load SMTP servers")
	}

	domains, _, err := h.domainRepo.FindByScope(scope, exportLimit, 0)
	if err != nil {
		return c.AbortInternalServerError("failed to load domains")
	}

	subscribers, _, err := h.subscriberRepo.FindByScope(scope, "", "", exportLimit, 0)
	if err != nil {
		return c.AbortInternalServerError("failed to load subscribers")
	}

	subscriberLists, _, err := h.subscriberListRepo.FindByScope(scope, exportLimit, 0)
	if err != nil {
		return c.AbortInternalServerError("failed to load subscriber lists")
	}

	return ok(c, WorkspaceDataExport{
		PostaVersion: config.Version,
		ExportedAt:   time.Now().UTC().Format(time.RFC3339),
		WorkspaceSettings: ExportWorkspaceSettings{
			Name:            ws.Name,
			Description:     ws.Description,
			DefaultLanguage: ws.DefaultLanguage,
		},
		Templates:       tmplExport,
		Stylesheets:     exportStylesheets(stylesheets),
		Languages:       exportLanguages(languages),
		Contacts:        exportContacts(contacts),
		ContactLists:    exportContactLists(contactLists, h.db),
		Suppressions:    exportSuppressions(suppressions),
		Webhooks:        exportWebhooks(webhooks),
		SMTPServers:     exportSMTPServers(smtpServers),
		Domains:         exportDomains(domains),
		Subscribers:     exportSubscribers(subscribers),
		SubscriberLists: exportSubscriberLists(subscriberLists),
	})
}

func (h *WorkspaceDataHandler) Import(c *okapi.Context, req *ImportWorkspaceDataRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("Insufficient workspace permissions", err)
	}
	userID := uint(c.GetInt("user_id"))
	wsID := uint(c.GetInt("workspace_id"))
	data := req.Body

	// Import workspace settings
	if data.WorkspaceSettings.Name != "" || data.WorkspaceSettings.Description != "" || data.WorkspaceSettings.DefaultLanguage != "" {
		ws, err := h.workspaceRepo.FindByID(wsID)
		if err == nil {
			if data.WorkspaceSettings.Name != "" {
				ws.Name = data.WorkspaceSettings.Name
			}
			if data.WorkspaceSettings.Description != "" {
				ws.Description = data.WorkspaceSettings.Description
			}
			if data.WorkspaceSettings.DefaultLanguage != "" {
				ws.DefaultLanguage = data.WorkspaceSettings.DefaultLanguage
			}
			ws.UpdatedAt = time.Now()
			_ = h.workspaceRepo.Update(ws)
		}
	}

	var total int
	total += importLanguages(data.Languages, userID, &wsID, h.languageRepo)
	total += importStylesheets(data.Stylesheets, userID, &wsID, h.stylesheetRepo)
	total += importTemplates(data.Templates, userID, &wsID, h.templateRepo, h.versionRepo, h.localizationRepo)
	total += importContacts(data.Contacts, userID, &wsID, h.db)
	total += importContactLists(data.ContactLists, userID, &wsID, h.db)
	total += importSuppressions(data.Suppressions, userID, &wsID, h.suppressionRepo)
	total += importWebhooks(data.Webhooks, userID, &wsID, h.db)
	total += importSMTPServers(data.SMTPServers, userID, &wsID, h.db)
	total += importDomains(data.Domains, userID, &wsID, h.db)
	total += importSubscribers(data.Subscribers, userID, &wsID, h.db)
	total += importSubscriberLists(data.SubscriberLists, userID, &wsID, h.db)

	return ok(c, map[string]any{
		"message":        "Workspace data imported successfully",
		"imported_count": total,
	})
}
