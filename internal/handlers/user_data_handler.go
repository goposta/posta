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

	"github.com/goposta/posta/internal/config"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
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
		webhookRepo:      webhookRepo,
		suppressionRepo:  suppressionRepo,
		userSettingRepo:  userSettingRepo,
	}
}

type UserDataExport struct {
	PostaVersion string              `json:"posta_version"`
	ExportedAt   string              `json:"exported_at"`
	Templates    []TemplateExport    `json:"templates"`
	Stylesheets  []ExportStyleSheet  `json:"stylesheets"`
	Languages    []ExportLanguage    `json:"languages"`
	Contacts     []ExportContact     `json:"contacts"`
	Suppressions []ExportSuppression `json:"suppressions"`
	Webhooks     []ExportWebhook     `json:"webhooks"`
	Settings     *ExportUserSettings `json:"settings,omitempty"`
}

type ImportUserDataRequest struct {
	Body UserDataExport `json:"body"`
}

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

func (h *UserDataHandler) Export(c *okapi.Context) error {
	userID := uint(c.GetInt("user_id"))
	scope := repositories.ResourceScope{UserID: userID}

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

	suppressions, _, err := h.suppressionRepo.FindByScope(scope, exportLimit, 0)
	if err != nil {
		return c.AbortInternalServerError("failed to load suppressions")
	}

	webhooks, _, err := h.webhookRepo.FindByScope(scope, exportLimit, 0)
	if err != nil {
		return c.AbortInternalServerError("failed to load webhooks")
	}

	var settings *ExportUserSettings
	s, err := h.userSettingRepo.FindByUserID(userID)
	if err == nil && s != nil {
		settings = &ExportUserSettings{
			Timezone:           s.Timezone,
			DefaultSenderName:  s.DefaultSenderName,
			DefaultSenderEmail: s.DefaultSenderEmail,
			EmailNotifications: s.EmailNotifications,
			NotificationEmail:  s.NotificationEmail,
			WebhookRetryCount:  s.WebhookRetryCount,
			APIKeyExpiryDays:   s.APIKeyExpiryDays,
			BounceAutoSuppress: s.BounceAutoSuppress,
			DailyReport:        s.DailyReport,
		}
	}

	return ok(c, UserDataExport{
		PostaVersion: config.Version,
		ExportedAt:   time.Now().UTC().Format(time.RFC3339),
		Templates:    tmplExport,
		Stylesheets:  exportStylesheets(stylesheets),
		Languages:    exportLanguages(languages),
		Contacts:     exportContacts(contacts),
		Suppressions: exportSuppressions(suppressions),
		Webhooks:     exportWebhooks(webhooks),
		Settings:     settings,
	})
}

func (h *UserDataHandler) Import(c *okapi.Context, req *ImportUserDataRequest) error {
	userID := uint(c.GetInt("user_id"))
	data := req.Body

	var total int
	total += importLanguages(data.Languages, userID, nil, h.languageRepo)
	total += importStylesheets(data.Stylesheets, userID, nil, h.stylesheetRepo)
	total += importTemplates(data.Templates, userID, nil, h.templateRepo, h.versionRepo, h.localizationRepo)
	total += importContacts(data.Contacts, userID, nil, h.db)
	total += importSuppressions(data.Suppressions, userID, nil, h.suppressionRepo)
	total += importWebhooks(data.Webhooks, userID, nil, h.db)

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
		"imported_count": total,
	})
}

func (h *UserDataHandler) DeleteContacts(c *okapi.Context, req *GDPRDeleteContactsRequest) error {
	userID := uint(c.GetInt("user_id"))
	email := req.Body.Email

	if email == "" {
		result := h.db.Where("user_id = ?", userID).Delete(&models.Contact{})
		if result.Error != nil {
			return c.AbortInternalServerError("failed to delete contacts")
		}
		return ok(c, GDPRDeleteResult{
			Deleted: result.RowsAffected,
			Message: "All contacts deleted",
		})
	}

	result := h.db.Where("user_id = ? AND email = ?", userID, email).Delete(&models.Contact{})
	if result.Error != nil {
		return c.AbortInternalServerError("failed to delete contact")
	}

	h.db.Where("user_id = ? AND email = ?", userID, email).Delete(&models.Suppression{})

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

func (h *UserDataHandler) DeleteEmailLogs(c *okapi.Context, req *GDPRDeleteEmailLogsRequest) error {
	userID := uint(c.GetInt("user_id"))

	days := req.Body.OlderThanDays
	if days <= 0 {
		days = 0
	}

	query := h.db.Where("user_id = ?", userID)
	if days > 0 {
		before := time.Now().AddDate(0, 0, -days)
		query = query.Where("created_at < ?", before)
	}

	var emailIDs []uint
	query.Model(&models.Email{}).Pluck("id", &emailIDs)
	if len(emailIDs) > 0 {
		h.db.Where("email_id IN ?", emailIDs).Delete(&models.Bounce{})
	}

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
