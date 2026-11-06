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

package notification

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"

	"github.com/goposta/posta/internal/config"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/email"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/logger"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

const (
	TemplateDailyReport     = "daily_report"
	TemplateInvitation      = "workspace_invitation"
	TemplateWelcome         = "welcome"
	TemplatePasswordChanged = "password_changed"
	TemplatePasswordReset   = "password_reset"
	TemplateAPIKeyCreated   = "api_key_created"
	TemplateAPIKeyExpiry    = "api_key_expiry"
	TemplateBounceAlert     = "bounce_alert"
	TemplateRoleChanged     = "role_changed"
	TemplateEmailVerify     = "email_verification"
	TemplateLoginAlert      = "login_alert"
	TemplateTwoFactorChange = "two_factor_changed"
	TemplateAccountDeletion = "account_deletion"
	TemplateNewMessage      = "new_message"
	TemplateMessageDigest   = "message_digest"
)

var templatePreference = map[string]func(*models.UserSetting) bool{
	TemplateDailyReport:   func(s *models.UserSetting) bool { return s.DailyReport },
	TemplateBounceAlert:   func(s *models.UserSetting) bool { return s.NotifyBounceAlerts },
	TemplateAPIKeyExpiry:  func(s *models.UserSetting) bool { return s.NotifyAPIKeyExpiry },
	TemplateRoleChanged:   func(s *models.UserSetting) bool { return s.NotifyWorkspaceActivity },
	TemplateNewMessage:    func(s *models.UserSetting) bool { return s.NotifyNewMessage },
	TemplateMessageDigest: func(s *models.UserSetting) bool { return s.NotifyNewMessage },
}

func templateEnabled(templateName string, settings *models.UserSetting) bool {
	if pref, ok := templatePreference[templateName]; ok {
		return pref(settings)
	}
	return true
}

type Service struct {
	smtpCfg         config.SystemSMTPConfig
	appName         string
	appURL          string
	sender          *email.SMTPSender
	userRepo        *repositories.UserRepository
	userSettingRepo *repositories.UserSettingRepository
	workspaceRepo   *repositories.WorkspaceRepository
	smtpRepo        *repositories.SMTPRepository
	templates       map[string]*template.Template
}

func NewService(
	smtpCfg config.SystemSMTPConfig,
	appName, appURL string,
	userRepo *repositories.UserRepository,
	userSettingRepo *repositories.UserSettingRepository,
	workspaceRepo *repositories.WorkspaceRepository,
) *Service {
	s := &Service{
		smtpCfg:         smtpCfg,
		appName:         appName,
		appURL:          appURL,
		sender:          email.NewSMTPSender(),
		userRepo:        userRepo,
		userSettingRepo: userSettingRepo,
		workspaceRepo:   workspaceRepo,
		templates:       make(map[string]*template.Template),
	}
	s.loadTemplates()
	return s
}

func (s *Service) loadTemplates() {
	names := []string{
		TemplateDailyReport,
		TemplateInvitation,
		TemplateWelcome,
		TemplatePasswordChanged,
		TemplatePasswordReset,
		TemplateAPIKeyCreated,
		TemplateAPIKeyExpiry,
		TemplateBounceAlert,
		TemplateRoleChanged,
		TemplateEmailVerify,
		TemplateLoginAlert,
		TemplateTwoFactorChange,
		TemplateAccountDeletion,
		TemplateNewMessage,
		TemplateMessageDigest,
	}
	for _, name := range names {
		tmpl := template.Must(template.ParseFS(templateFS,
			"templates/base.tmpl",
			fmt.Sprintf("templates/%s.tmpl", name),
		))
		s.templates[name] = tmpl
	}
}

func (s *Service) IsConfigured() bool {
	return s.smtpCfg.IsConfigured()
}

func (s *Service) Send(to, subject, templateName string, data map[string]any) error {
	if !s.IsConfigured() {
		logger.Debug("notification service: system SMTP not configured, skipping", "template", templateName, "to", to)
		return nil
	}

	if data == nil {
		data = make(map[string]any)
	}
	data["AppName"] = s.appName
	data["AppURL"] = s.appURL
	data["Subject"] = subject

	html, err := s.render(templateName, data)
	if err != nil {
		return fmt.Errorf("notification render %s: %w", templateName, err)
	}

	server := s.systemServer()
	if err := s.sender.Send(server, s.smtpCfg.From, []string{to}, subject, html, "", nil, nil, "", "", false); err != nil {
		return fmt.Errorf("notification send to %s: %w", to, err)
	}

	return nil
}

// SendToUser sends a notification to a user, respecting their notification preferences.
func (s *Service) SendToUser(userID uint, subject, templateName string, data map[string]any) error {
	return s.sendToUser(userID, subject, templateName, data, false)
}

// SendSecurityToUser ignores the EmailNotifications opt-out: security mail must always reach the owner.
func (s *Service) SendSecurityToUser(userID uint, subject, templateName string, data map[string]any) error {
	return s.sendToUser(userID, subject, templateName, data, true)
}

// sendToUser ignores the recipient's EmailNotifications toggle when security is true.
func (s *Service) sendToUser(userID uint, subject, templateName string, data map[string]any, security bool) error {
	if !s.IsConfigured() {
		return nil
	}

	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return fmt.Errorf("notification: user %d not found: %w", userID, err)
	}

	settings, err := s.userSettingRepo.FindByUserID(userID)
	if err != nil {
		return fmt.Errorf("notification: settings for user %d: %w", userID, err)
	}

	if !security && !settings.EmailNotifications {
		logger.Debug("notification service: email notifications disabled for user", "user_id", userID)
		return nil
	}

	if !security && !templateEnabled(templateName, settings) {
		logger.Debug("notification service: notification type disabled for user", "user_id", userID, "template", templateName)
		return nil
	}

	if templateName != TemplateEmailVerify && user.EmailVerifiedAt == nil {
		logger.Debug("notification service: skipping send to unverified user", "user_id", userID, "template", templateName)
		return nil
	}

	to := settings.NotificationEmail
	if to == "" {
		to = user.Email
	}

	if data == nil {
		data = make(map[string]any)
	}
	data["UserName"] = user.Name
	if data["UserName"] == "" {
		data["UserName"] = user.Email
	}

	return s.Send(to, subject, templateName, data)
}

func (s *Service) SendToWorkspaceAdmins(workspaceID uint, subject, templateName string, data map[string]any) error {
	if !s.IsConfigured() {
		return nil
	}

	members, err := s.workspaceRepo.ListMembers(workspaceID)
	if err != nil {
		return fmt.Errorf("notification: members for workspace %d: %w", workspaceID, err)
	}

	if data == nil {
		data = make(map[string]any)
	}
	if _, ok := data["WorkspaceName"]; !ok {
		if ws, err := s.workspaceRepo.FindByID(workspaceID); err == nil {
			data["WorkspaceName"] = ws.DisplayName
		}
	}

	var firstErr error
	for _, m := range members {
		if m.Role != models.WorkspaceRoleOwner && m.Role != models.WorkspaceRoleAdmin {
			continue
		}
		if err := s.SendToUser(m.UserID, subject, templateName, data); err != nil {
			logger.Error("notification: failed to send to workspace admin",
				"workspace_id", workspaceID, "user_id", m.UserID, "error", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (s *Service) render(templateName string, data map[string]any) (string, error) {
	tmpl, ok := s.templates[templateName]
	if !ok {
		return "", fmt.Errorf("unknown notification template: %s", templateName)
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "base", data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// systemServer prefers the provisioned system row; config is the fallback (worker has no repo, first boot).
func (s *Service) systemServer() *models.SMTPServer {
	if s.smtpRepo != nil {
		if server, err := s.smtpRepo.FindSystem(); err == nil && server.Host != "" {
			return server
		}
	}
	return &models.SMTPServer{
		Host:       s.smtpCfg.Host,
		Port:       s.smtpCfg.Port,
		Username:   s.smtpCfg.Username,
		Password:   s.smtpCfg.Password,
		Encryption: s.smtpCfg.Encryption,
	}
}

// SetSMTPRepo is optional: without it the service sends straight from configuration.
func (s *Service) SetSMTPRepo(repo *repositories.SMTPRepository) { s.smtpRepo = repo }
