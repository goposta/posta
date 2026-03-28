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
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/storage/repositories"
	"gorm.io/gorm"
)

// exportLimit is the maximum number of records to export per resource type.
const exportLimit = 10000


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
	Members     []ExportContactListMember `json:"members"`
}

type ExportContactListMember struct {
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

type ExportSMTPServer struct {
	Host          string   `json:"host"`
	Port          int      `json:"port"`
	Username      string   `json:"username"`
	Encryption    string   `json:"encryption"`
	MaxRetries    int      `json:"max_retries"`
	AllowedEmails []string `json:"allowed_emails,omitempty"`
}

type ExportDomain struct {
	Domain string `json:"domain"`
}

type ExportSubscriber struct {
	Email        string                 `json:"email"`
	Name         string                 `json:"name"`
	Status       string                 `json:"status"`
	CustomFields map[string]interface{} `json:"custom_fields,omitempty"`
	Timezone     string                 `json:"timezone,omitempty"`
	Language     string                 `json:"language,omitempty"`
}

type ExportSubscriberList struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Type        string              `json:"type"`
	FilterRules []models.FilterRule `json:"filter_rules,omitempty"`
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

type ExportWorkspaceSettings struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	DefaultLanguage string `json:"default_language"`
}


func exportTemplates(
	templates []models.Template,
	versionRepo *repositories.TemplateVersionRepository,
	localizationRepo *repositories.TemplateLocalizationRepository,
) ([]TemplateExport, error) {
	result := make([]TemplateExport, 0, len(templates))
	for _, tmpl := range templates {
		versions, err := versionRepo.FindByTemplateID(tmpl.ID)
		if err != nil {
			return nil, err
		}

		exportVersions := make([]ExportVersion, 0, len(versions))
		for _, v := range versions {
			localizations, err := localizationRepo.FindByVersionID(v.ID)
			if err != nil {
				return nil, err
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

		result = append(result, TemplateExport{
			Name:            tmpl.Name,
			Description:     tmpl.Description,
			DefaultLanguage: tmpl.DefaultLanguage,
			SampleData:      tmpl.SampleData,
			Versions:        exportVersions,
		})
	}
	return result, nil
}

func exportStylesheets(stylesheets []models.StyleSheet) []ExportStyleSheet {
	result := make([]ExportStyleSheet, 0, len(stylesheets))
	for _, ss := range stylesheets {
		result = append(result, ExportStyleSheet{Name: ss.Name, CSS: ss.CSS})
	}
	return result
}

func exportLanguages(languages []models.Language) []ExportLanguage {
	result := make([]ExportLanguage, 0, len(languages))
	for _, l := range languages {
		result = append(result, ExportLanguage{Code: l.Code, Name: l.Name})
	}
	return result
}

func exportContacts(contacts []models.Contact) []ExportContact {
	result := make([]ExportContact, 0, len(contacts))
	for _, c := range contacts {
		result = append(result, ExportContact{
			Email: c.Email, Name: c.Name,
			SentCount: c.SentCount, FailCount: c.FailCount,
		})
	}
	return result
}

func exportContactLists(contactLists []models.ContactList, db *gorm.DB) []ExportContactList {
	result := make([]ExportContactList, 0, len(contactLists))
	for _, cl := range contactLists {
		var members []models.ContactListMember
		db.Where("list_id = ?", cl.ID).Find(&members)

		exportMembers := make([]ExportContactListMember, 0, len(members))
		for _, m := range members {
			exportMembers = append(exportMembers, ExportContactListMember{
				Email: m.Email, Name: m.Name, Data: m.Data,
			})
		}
		result = append(result, ExportContactList{
			Name: cl.Name, Description: cl.Description, Members: exportMembers,
		})
	}
	return result
}

func exportSuppressions(suppressions []models.Suppression) []ExportSuppression {
	result := make([]ExportSuppression, 0, len(suppressions))
	for _, s := range suppressions {
		result = append(result, ExportSuppression{Email: s.Email, Reason: s.Reason})
	}
	return result
}

func exportWebhooks(webhooks []models.Webhook) []ExportWebhook {
	result := make([]ExportWebhook, 0, len(webhooks))
	for _, wh := range webhooks {
		result = append(result, ExportWebhook{
			URL: wh.URL, Events: wh.Events, Filters: wh.Filters,
		})
	}
	return result
}

func exportSMTPServers(servers []models.SMTPServer) []ExportSMTPServer {
	result := make([]ExportSMTPServer, 0, len(servers))
	for _, s := range servers {
		result = append(result, ExportSMTPServer{
			Host: s.Host, Port: s.Port, Username: s.Username,
			Encryption: s.Encryption, MaxRetries: s.MaxRetries,
			AllowedEmails: s.AllowedEmails,
		})
	}
	return result
}

func exportDomains(domains []models.Domain) []ExportDomain {
	result := make([]ExportDomain, 0, len(domains))
	for _, d := range domains {
		result = append(result, ExportDomain{Domain: d.Domain})
	}
	return result
}

func exportSubscribers(subscribers []models.Subscriber) []ExportSubscriber {
	result := make([]ExportSubscriber, 0, len(subscribers))
	for _, s := range subscribers {
		result = append(result, ExportSubscriber{
			Email: s.Email, Name: s.Name, Status: string(s.Status),
			CustomFields: s.CustomFields, Timezone: s.Timezone, Language: s.Language,
		})
	}
	return result
}

func exportSubscriberLists(lists []models.SubscriberList) []ExportSubscriberList {
	result := make([]ExportSubscriberList, 0, len(lists))
	for _, sl := range lists {
		result = append(result, ExportSubscriberList{
			Name: sl.Name, Description: sl.Description,
			Type: string(sl.Type), FilterRules: sl.FilterRules,
		})
	}
	return result
}


func importTemplates(
	data []TemplateExport,
	userID uint,
	workspaceID *uint,
	templateRepo *repositories.TemplateRepository,
	versionRepo *repositories.TemplateVersionRepository,
	localizationRepo *repositories.TemplateLocalizationRepository,
) int {
	var count int
	for _, tmplData := range data {
		if tmplData.Name == "" {
			continue
		}

		defaultLang := tmplData.DefaultLanguage
		if defaultLang == "" {
			defaultLang = "en"
		}

		tmpl := &models.Template{
			UserID:          userID,
			WorkspaceID:     workspaceID,
			Name:            tmplData.Name,
			DefaultLanguage: defaultLang,
			Description:     tmplData.Description,
			SampleData:      tmplData.SampleData,
		}
		if err := templateRepo.Create(tmpl); err != nil {
			continue
		}

		var activeVersionDBID *uint
		for _, ev := range tmplData.Versions {
			nextVersion, err := versionRepo.NextVersion(tmpl.ID)
			if err != nil {
				continue
			}
			v := &models.TemplateVersion{
				TemplateID: tmpl.ID,
				Version:    nextVersion,
				SampleData: ev.SampleData,
			}
			if err := versionRepo.Create(v); err != nil {
				continue
			}
			if ev.IsActive {
				activeVersionDBID = &v.ID
			}
			for _, el := range ev.Localizations {
				_ = localizationRepo.Create(&models.TemplateLocalization{
					VersionID:       v.ID,
					Language:        el.Language,
					SubjectTemplate: el.SubjectTemplate,
					HTMLTemplate:    el.HTMLTemplate,
					TextTemplate:    el.TextTemplate,
				})
			}
		}

		if len(tmplData.Versions) == 0 {
			v := &models.TemplateVersion{
				TemplateID: tmpl.ID, Version: 1, SampleData: tmplData.SampleData,
			}
			if err := versionRepo.Create(v); err == nil {
				activeVersionDBID = &v.ID
			}
		}

		if activeVersionDBID != nil {
			tmpl.ActiveVersionID = activeVersionDBID
			_ = templateRepo.Update(tmpl)
		}
		count++
	}
	return count
}

func importLanguages(data []ExportLanguage, userID uint, workspaceID *uint, repo *repositories.LanguageRepository) int {
	var count int
	for _, lang := range data {
		if lang.Code == "" || lang.Name == "" {
			continue
		}
		if err := repo.Create(&models.Language{
			UserID: userID, WorkspaceID: workspaceID,
			Code: lang.Code, Name: lang.Name,
		}); err != nil {
			continue
		}
		count++
	}
	return count
}

func importStylesheets(data []ExportStyleSheet, userID uint, workspaceID *uint, repo *repositories.StyleSheetRepository) int {
	var count int
	for _, ss := range data {
		if ss.Name == "" {
			continue
		}
		if err := repo.Create(&models.StyleSheet{
			UserID: userID, WorkspaceID: workspaceID,
			Name: ss.Name, CSS: ss.CSS,
		}); err != nil {
			continue
		}
		count++
	}
	return count
}

func importContacts(data []ExportContact, userID uint, workspaceID *uint, db *gorm.DB) int {
	var count int
	for _, ct := range data {
		if ct.Email == "" {
			continue
		}
		if err := db.Create(&models.Contact{
			UserID: userID, WorkspaceID: workspaceID,
			Email: ct.Email, Name: ct.Name,
			SentCount: ct.SentCount, FailCount: ct.FailCount,
		}).Error; err != nil {
			continue
		}
		count++
	}
	return count
}

func importContactLists(data []ExportContactList, userID uint, workspaceID *uint, db *gorm.DB) int {
	var count int
	for _, cl := range data {
		if cl.Name == "" {
			continue
		}
		list := &models.ContactList{
			UserID: userID, WorkspaceID: workspaceID,
			Name: cl.Name, Description: cl.Description,
		}
		if err := db.Create(list).Error; err != nil {
			continue
		}
		for _, m := range cl.Members {
			if m.Email == "" {
				continue
			}
			db.Create(&models.ContactListMember{
				ListID: list.ID, Email: m.Email, Name: m.Name, Data: m.Data,
			})
		}
		count++
	}
	return count
}

func importSuppressions(data []ExportSuppression, userID uint, workspaceID *uint, repo *repositories.SuppressionRepository) int {
	var count int
	for _, s := range data {
		if s.Email == "" {
			continue
		}
		if err := repo.Create(&models.Suppression{
			UserID: userID, WorkspaceID: workspaceID,
			Email: s.Email, Reason: s.Reason,
		}); err != nil {
			continue
		}
		count++
	}
	return count
}

func importWebhooks(data []ExportWebhook, userID uint, workspaceID *uint, db *gorm.DB) int {
	var count int
	for _, wh := range data {
		if wh.URL == "" {
			continue
		}
		if err := db.Create(&models.Webhook{
			UserID: userID, WorkspaceID: workspaceID,
			URL: wh.URL, Events: wh.Events, Filters: wh.Filters,
		}).Error; err != nil {
			continue
		}
		count++
	}
	return count
}

func importSMTPServers(data []ExportSMTPServer, userID uint, workspaceID *uint, db *gorm.DB) int {
	var count int
	for _, s := range data {
		if s.Host == "" {
			continue
		}
		if err := db.Create(&models.SMTPServer{
			UserID: userID, WorkspaceID: workspaceID,
			Host: s.Host, Port: s.Port, Username: s.Username,
			Encryption: s.Encryption, MaxRetries: s.MaxRetries,
			AllowedEmails: s.AllowedEmails,
			Status: models.SMTPStatusDisabled,
		}).Error; err != nil {
			continue
		}
		count++
	}
	return count
}

func importDomains(data []ExportDomain, userID uint, workspaceID *uint, db *gorm.DB) int {
	var count int
	for _, d := range data {
		if d.Domain == "" {
			continue
		}
		token, err := generateToken()
		if err != nil {
			continue
		}
		if err := db.Create(&models.Domain{
			UserID: userID, WorkspaceID: workspaceID,
			Domain: d.Domain, VerificationToken: token,
		}).Error; err != nil {
			continue
		}
		count++
	}
	return count
}

func importSubscribers(data []ExportSubscriber, userID uint, workspaceID *uint, db *gorm.DB) int {
	var count int
	for _, sub := range data {
		if sub.Email == "" {
			continue
		}
		status := models.SubscriberStatus(sub.Status)
		if status == "" {
			status = models.SubscriberStatusSubscribed
		}
		if err := db.Create(&models.Subscriber{
			UserID: userID, WorkspaceID: workspaceID,
			Email: sub.Email, Name: sub.Name, Status: status,
			CustomFields: sub.CustomFields, Timezone: sub.Timezone, Language: sub.Language,
		}).Error; err != nil {
			continue
		}
		count++
	}
	return count
}

func importSubscriberLists(data []ExportSubscriberList, userID uint, workspaceID *uint, db *gorm.DB) int {
	var count int
	for _, sl := range data {
		if sl.Name == "" {
			continue
		}
		listType := models.SubscriberListType(sl.Type)
		if listType == "" {
			listType = models.SubscriberListTypeStatic
		}
		if err := db.Create(&models.SubscriberList{
			UserID: userID, WorkspaceID: workspaceID,
			Name: sl.Name, Description: sl.Description,
			Type: listType, FilterRules: sl.FilterRules,
		}).Error; err != nil {
			continue
		}
		count++
	}
	return count
}
