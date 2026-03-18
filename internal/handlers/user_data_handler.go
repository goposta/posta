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
	"github.com/jkaninda/posta/internal/storage/repositories"
	"gorm.io/gorm"
)

type UserDataHandler struct {
	db               *gorm.DB
	templateRepo     *repositories.TemplateRepository
	versionRepo      *repositories.TemplateVersionRepository
	localizationRepo *repositories.TemplateLocalizationRepository
	stylesheetRepo   *repositories.StyleSheetRepository
	languageRepo     *repositories.LanguageRepository
	contactRepo      *repositories.ContactRepository
	contactListRepo  *repositories.ContactListRepository
	webhookRepo      *repositories.WebhookRepository
	suppressionRepo  *repositories.SuppressionRepository
	userSettingRepo  *repositories.UserSettingRepository
}

func NewUserDataHandler(
	db *gorm.DB,
	templateRepo *repositories.TemplateRepository,
	versionRepo *repositories.TemplateVersionRepository,
	localizationRepo *repositories.TemplateLocalizationRepository,
	stylesheetRepo *repositories.StyleSheetRepository,
	languageRepo *repositories.LanguageRepository,
	contactRepo *repositories.ContactRepository,
	contactListRepo *repositories.ContactListRepository,
	webhookRepo *repositories.WebhookRepository,
	suppressionRepo *repositories.SuppressionRepository,
	userSettingRepo *repositories.UserSettingRepository,
) *UserDataHandler {
	return &UserDataHandler{
		db:               db,
		templateRepo:     templateRepo,
		versionRepo:      versionRepo,
		localizationRepo: localizationRepo,
		stylesheetRepo:   stylesheetRepo,
		languageRepo:     languageRepo,
		contactRepo:      contactRepo,
		contactListRepo:  contactListRepo,
		webhookRepo:      webhookRepo,
		suppressionRepo:  suppressionRepo,
		userSettingRepo:  userSettingRepo,
	}
}

// --- Export / Import types ---

type UserDataExport struct {
	PostaVersion string              `json:"posta_version"`
	ExportedAt   string              `json:"exported_at"`
	Templates    []TemplateExport    `json:"templates"`
	Stylesheets  []ExportStyleSheet  `json:"stylesheets"`
	Languages    []ExportLanguage    `json:"languages"`
	Contacts     []ExportContact     `json:"contacts"`
	ContactLists []ExportContactList `json:"contact_lists"`
	Suppressions []ExportSuppression `json:"suppressions"`
	Webhooks     []ExportWebhook     `json:"webhooks"`
	Settings     *ExportUserSettings `json:"settings,omitempty"`
}

type ExportStyleSheet struct {
	Name string `json:"name"`
	CSS  string `json:"css"`
}

type ExportLanguage struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type ExportContact struct {
	Email     string `json:"email"`
	Name      string `json:"name"`
	SentCount int64  `json:"sent_count"`
	FailCount int64  `json:"fail_count"`
}

type ExportContactList struct {
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Members     []ExportContactMember `json:"members"`
}

type ExportContactMember struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	Data  string `json:"data,omitempty"`
}

type ExportSuppression struct {
	Email  string `json:"email"`
	Reason string `json:"reason"`
}

type ExportWebhook struct {
	URL     string   `json:"url"`
	Events  []string `json:"events"`
	Filters []string `json:"filters,omitempty"`
}

type ExportUserSettings struct {
	Timezone           string `json:"timezone"`
	DefaultSenderName  string `json:"default_sender_name"`
	DefaultSenderEmail string `json:"default_sender_email"`
	EmailNotifications bool   `json:"email_notifications"`
	NotificationEmail  string `json:"notification_email"`
	WebhookRetryCount  int    `json:"webhook_retry_count"`
	APIKeyExpiryDays   int    `json:"api_key_expiry_days"`
	BounceAutoSuppress bool   `json:"bounce_auto_suppress"`
	DailyReport        bool   `json:"daily_report"`
}

type ImportUserDataRequest struct {
	Body UserDataExport `json:"body"`
}

// --- GDPR types ---

type GDPRDeleteContactsRequest struct {
	Body struct {
		Email string `json:"email"`
	} `json:"body"`
}

type GDPRDeleteEmailLogsRequest struct {
	Body struct {
		OlderThanDays int `json:"older_than_days"`
	} `json:"body"`
}

type GDPRDeleteResult struct {
	Deleted int64  `json:"deleted"`
	Message string `json:"message"`
}

// --- Export handler ---

func (h *UserDataHandler) Export(c *okapi.Context) error {
	userID := uint(c.GetInt("user_id"))

	// Templates (reuse existing TemplateExport struct)
	templates, _, err := h.templateRepo.FindByUserID(userID, 10000, 0)
	if err != nil {
		return c.AbortInternalServerError("failed to load templates")
	}

	exportTemplates := make([]TemplateExport, 0, len(templates))
	for _, tmpl := range templates {
		versions, err := h.versionRepo.FindByTemplateID(tmpl.ID)
		if err != nil {
			return c.AbortInternalServerError("failed to load template versions")
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

		exportTemplates = append(exportTemplates, TemplateExport{
			Name:            tmpl.Name,
			Description:     tmpl.Description,
			DefaultLanguage: tmpl.DefaultLanguage,
			SampleData:      tmpl.SampleData,
			Versions:        exportVersions,
		})
	}

	// Stylesheets
	stylesheets, _, err := h.stylesheetRepo.FindByUserID(userID, 10000, 0)
	if err != nil {
		return c.AbortInternalServerError("failed to load stylesheets")
	}
	exportStylesheets := make([]ExportStyleSheet, 0, len(stylesheets))
	for _, ss := range stylesheets {
		exportStylesheets = append(exportStylesheets, ExportStyleSheet{
			Name: ss.Name,
			CSS:  ss.CSS,
		})
	}

	// Languages
	languages, _, err := h.languageRepo.FindByUserID(userID, 10000, 0)
	if err != nil {
		return c.AbortInternalServerError("failed to load languages")
	}
	exportLanguages := make([]ExportLanguage, 0, len(languages))
	for _, lang := range languages {
		exportLanguages = append(exportLanguages, ExportLanguage{
			Code: lang.Code,
			Name: lang.Name,
		})
	}

	// Contacts
	contacts, _, err := h.contactRepo.FindByUserID(userID, "", 10000, 0)
	if err != nil {
		return c.AbortInternalServerError("failed to load contacts")
	}
	exportContacts := make([]ExportContact, 0, len(contacts))
	for _, ct := range contacts {
		exportContacts = append(exportContacts, ExportContact{
			Email:     ct.Email,
			Name:      ct.Name,
			SentCount: ct.SentCount,
			FailCount: ct.FailCount,
		})
	}

	// Contact Lists with members
	contactLists, _, err := h.contactListRepo.FindByUserID(userID, 10000, 0)
	if err != nil {
		return c.AbortInternalServerError("failed to load contact lists")
	}
	exportContactLists := make([]ExportContactList, 0, len(contactLists))
	for _, cl := range contactLists {
		members, _, err := h.contactListRepo.ListMembers(cl.ID, 100000, 0)
		if err != nil {
			return c.AbortInternalServerError("failed to load contact list members")
		}
		exportMembers := make([]ExportContactMember, 0, len(members))
		for _, m := range members {
			exportMembers = append(exportMembers, ExportContactMember{
				Email: m.Email,
				Name:  m.Name,
				Data:  m.Data,
			})
		}
		exportContactLists = append(exportContactLists, ExportContactList{
			Name:        cl.Name,
			Description: cl.Description,
			Members:     exportMembers,
		})
	}

	// Suppressions
	suppressions, _, err := h.suppressionRepo.FindByUserID(userID, 10000, 0)
	if err != nil {
		return c.AbortInternalServerError("failed to load suppressions")
	}
	exportSuppressions := make([]ExportSuppression, 0, len(suppressions))
	for _, s := range suppressions {
		exportSuppressions = append(exportSuppressions, ExportSuppression{
			Email:  s.Email,
			Reason: s.Reason,
		})
	}

	// Webhooks
	webhooks, _, err := h.webhookRepo.FindByUserID(userID, 10000, 0)
	if err != nil {
		return c.AbortInternalServerError("failed to load webhooks")
	}
	exportWebhooks := make([]ExportWebhook, 0, len(webhooks))
	for _, wh := range webhooks {
		exportWebhooks = append(exportWebhooks, ExportWebhook{
			URL:     wh.URL,
			Events:  wh.Events,
			Filters: wh.Filters,
		})
	}

	// User settings
	var exportSettings *ExportUserSettings
	settings, err := h.userSettingRepo.FindByUserID(userID)
	if err == nil && settings != nil {
		exportSettings = &ExportUserSettings{
			Timezone:           settings.Timezone,
			DefaultSenderName:  settings.DefaultSenderName,
			DefaultSenderEmail: settings.DefaultSenderEmail,
			EmailNotifications: settings.EmailNotifications,
			NotificationEmail:  settings.NotificationEmail,
			WebhookRetryCount:  settings.WebhookRetryCount,
			APIKeyExpiryDays:   settings.APIKeyExpiryDays,
			BounceAutoSuppress: settings.BounceAutoSuppress,
			DailyReport:        settings.DailyReport,
		}
	}

	export := UserDataExport{
		PostaVersion: config.Version,
		ExportedAt:   time.Now().UTC().Format(time.RFC3339),
		Templates:    exportTemplates,
		Stylesheets:  exportStylesheets,
		Languages:    exportLanguages,
		Contacts:     exportContacts,
		ContactLists: exportContactLists,
		Suppressions: exportSuppressions,
		Webhooks:     exportWebhooks,
		Settings:     exportSettings,
	}

	return ok(c, export)
}

//  Import

func (h *UserDataHandler) Import(c *okapi.Context, req *ImportUserDataRequest) error {
	userID := uint(c.GetInt("user_id"))
	data := req.Body

	var importedCount int

	for _, lang := range data.Languages {
		if lang.Code == "" || lang.Name == "" {
			continue
		}
		l := &models.Language{
			UserID: userID,
			Code:   lang.Code,
			Name:   lang.Name,
		}
		if err := h.languageRepo.Create(l); err != nil {
			continue // skip duplicates
		}
		importedCount++
	}

	// Import stylesheets
	stylesheetMap := make(map[string]uint) // name -> id for template reference
	for _, ss := range data.Stylesheets {
		if ss.Name == "" {
			continue
		}
		s := &models.StyleSheet{
			UserID: userID,
			Name:   ss.Name,
			CSS:    ss.CSS,
		}
		if err := h.stylesheetRepo.Create(s); err != nil {
			continue // skip duplicates
		}
		stylesheetMap[s.Name] = s.ID
		importedCount++
	}

	// Import templates (reuse template import logic)
	for _, tmplData := range data.Templates {
		if tmplData.Name == "" {
			continue
		}

		defaultLang := tmplData.DefaultLanguage
		if defaultLang == "" {
			defaultLang = "en"
		}

		tmpl := &models.Template{
			UserID:          userID,
			Name:            tmplData.Name,
			DefaultLanguage: defaultLang,
			Description:     tmplData.Description,
			SampleData:      tmplData.SampleData,
		}

		if err := h.templateRepo.Create(tmpl); err != nil {
			continue // skip duplicates
		}

		var activeVersionDBID *uint

		for _, ev := range tmplData.Versions {
			nextVersion, err := h.versionRepo.NextVersion(tmpl.ID)
			if err != nil {
				continue
			}

			v := &models.TemplateVersion{
				TemplateID: tmpl.ID,
				Version:    nextVersion,
				SampleData: ev.SampleData,
			}
			if err := h.versionRepo.Create(v); err != nil {
				continue
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
				_ = h.localizationRepo.Create(l)
			}
		}

		if len(tmplData.Versions) == 0 {
			v := &models.TemplateVersion{
				TemplateID: tmpl.ID,
				Version:    1,
				SampleData: tmplData.SampleData,
			}
			if err := h.versionRepo.Create(v); err == nil {
				activeVersionDBID = &v.ID
			}
		}

		if activeVersionDBID != nil {
			tmpl.ActiveVersionID = activeVersionDBID
			_ = h.templateRepo.Update(tmpl)
		}

		importedCount++
	}

	// Import contacts
	for _, ct := range data.Contacts {
		if ct.Email == "" {
			continue
		}
		contact := &models.Contact{
			UserID:    userID,
			Email:     ct.Email,
			Name:      ct.Name,
			SentCount: ct.SentCount,
			FailCount: ct.FailCount,
		}
		if err := h.db.Create(contact).Error; err != nil {
			continue // skip duplicates
		}
		importedCount++
	}

	// Import contact lists
	for _, cl := range data.ContactLists {
		if cl.Name == "" {
			continue
		}
		list := &models.ContactList{
			UserID:      userID,
			Name:        cl.Name,
			Description: cl.Description,
		}
		if err := h.contactListRepo.Create(list); err != nil {
			continue
		}
		for _, m := range cl.Members {
			member := &models.ContactListMember{
				ListID: list.ID,
				Email:  m.Email,
				Name:   m.Name,
				Data:   m.Data,
			}
			_ = h.contactListRepo.AddMember(member)
		}
		importedCount++
	}

	// Import suppressions
	for _, s := range data.Suppressions {
		if s.Email == "" {
			continue
		}
		sup := &models.Suppression{
			UserID: userID,
			Email:  s.Email,
			Reason: s.Reason,
		}
		_ = h.suppressionRepo.Create(sup)
	}

	// Import webhooks
	for _, wh := range data.Webhooks {
		if wh.URL == "" {
			continue
		}
		webhook := &models.Webhook{
			UserID:  userID,
			URL:     wh.URL,
			Events:  wh.Events,
			Filters: wh.Filters,
		}
		h.db.Create(webhook)
	}

	// Import settings
	if data.Settings != nil {
		settings, err := h.userSettingRepo.FindByUserID(userID)
		if err == nil && settings != nil {
			settings.Timezone = data.Settings.Timezone
			settings.DefaultSenderName = data.Settings.DefaultSenderName
			settings.DefaultSenderEmail = data.Settings.DefaultSenderEmail
			settings.EmailNotifications = data.Settings.EmailNotifications
			settings.NotificationEmail = data.Settings.NotificationEmail
			settings.WebhookRetryCount = data.Settings.WebhookRetryCount
			settings.APIKeyExpiryDays = data.Settings.APIKeyExpiryDays
			settings.BounceAutoSuppress = data.Settings.BounceAutoSuppress
			settings.DailyReport = data.Settings.DailyReport
			_ = h.userSettingRepo.CreateOrUpdate(settings)
		}
	}

	return ok(c, map[string]any{
		"message":        "Data imported successfully",
		"imported_count": importedCount,
	})
}

//  GDPR Delete Contacts

func (h *UserDataHandler) DeleteContacts(c *okapi.Context, req *GDPRDeleteContactsRequest) error {
	userID := uint(c.GetInt("user_id"))
	email := req.Body.Email

	if email == "" {
		// Delete all contacts
		result := h.db.Where("user_id = ?", userID).Delete(&models.Contact{})
		if result.Error != nil {
			return c.AbortInternalServerError("failed to delete contacts")
		}
		return ok(c, GDPRDeleteResult{
			Deleted: result.RowsAffected,
			Message: "All contacts deleted",
		})
	}

	// Delete specific contact by email
	result := h.db.Where("user_id = ? AND email = ?", userID, email).Delete(&models.Contact{})
	if result.Error != nil {
		return c.AbortInternalServerError("failed to delete contact")
	}

	// Also remove from suppression list
	h.db.Where("user_id = ? AND email = ?", userID, email).Delete(&models.Suppression{})

	// Also remove from contact list memberships
	var listIDs []uint
	h.db.Model(&models.ContactList{}).Where("user_id = ?", userID).Pluck("id", &listIDs)
	if len(listIDs) > 0 {
		h.db.Where("list_id IN ? AND email = ?", listIDs, email).Delete(&models.ContactListMember{})
	}

	return ok(c, GDPRDeleteResult{
		Deleted: result.RowsAffected,
		Message: fmt.Sprintf("Contact %s and associated data deleted", email),
	})
}

// GDPR Delete Email Logs

func (h *UserDataHandler) DeleteEmailLogs(c *okapi.Context, req *GDPRDeleteEmailLogsRequest) error {
	userID := uint(c.GetInt("user_id"))

	days := req.Body.OlderThanDays
	if days <= 0 {
		days = 0 // delete all
	}

	query := h.db.Where("user_id = ?", userID)
	if days > 0 {
		before := time.Now().AddDate(0, 0, -days)
		query = query.Where("created_at < ?", before)
	}

	// Delete bounces for matching emails first
	var emailIDs []uint
	query.Model(&models.Email{}).Pluck("id", &emailIDs)
	if len(emailIDs) > 0 {
		h.db.Where("email_id IN ?", emailIDs).Delete(&models.Bounce{})
	}

	// Delete the emails
	result := query.Delete(&models.Email{})
	if result.Error != nil {
		return c.AbortInternalServerError("failed to delete email logs")
	}

	msg := "All email logs deleted"
	if days > 0 {
		msg = fmt.Sprintf("Email logs older than %d days deleted", days)
	}

	return ok(c, GDPRDeleteResult{
		Deleted: result.RowsAffected,
		Message: msg,
	})
}
