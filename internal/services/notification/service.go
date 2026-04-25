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

// Template names used throughout the notification system.
const (
	TemplateDailyReport     = "daily_report"
	TemplateInvitation      = "workspace_invitation"
	TemplateWelcome         = "welcome"
	TemplatePasswordChanged = "password_changed"
	TemplateAPIKeyCreated   = "api_key_created"
	TemplateAPIKeyExpiry    = "api_key_expiry"
	TemplateBounceAlert     = "bounce_alert"
	TemplateRoleChanged     = "role_changed"
	TemplateEmailVerify     = "email_verification"
)

// Service sends platform notification emails via the system SMTP server.
type Service struct {
	smtpCfg         config.SystemSMTPConfig
	appName         string
	appURL          string
	sender          *email.SMTPSender
	userRepo        *repositories.UserRepository
	userSettingRepo *repositories.UserSettingRepository
	templates       map[string]*template.Template
}

// NewService creates a new notification service.
func NewService(
	smtpCfg config.SystemSMTPConfig,
	appName, appURL string,
	userRepo *repositories.UserRepository,
	userSettingRepo *repositories.UserSettingRepository,
) *Service {
	s := &Service{
		smtpCfg:         smtpCfg,
		appName:         appName,
		appURL:          appURL,
		sender:          email.NewSMTPSender(),
		userRepo:        userRepo,
		userSettingRepo: userSettingRepo,
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
		TemplateAPIKeyCreated,
		TemplateAPIKeyExpiry,
		TemplateBounceAlert,
		TemplateRoleChanged,
		TemplateEmailVerify,
	}
	for _, name := range names {
		tmpl := template.Must(template.ParseFS(templateFS,
			"templates/base.tmpl",
			fmt.Sprintf("templates/%s.tmpl", name),
		))
		s.templates[name] = tmpl
	}
}

// IsConfigured returns true if the system SMTP is configured.
func (s *Service) IsConfigured() bool {
	return s.smtpCfg.IsConfigured()
}

// Send renders a template and sends an email to the given address.
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
	if err := s.sender.Send(server, s.smtpCfg.From, []string{to}, subject, html, "", nil, nil, "", false); err != nil {
		return fmt.Errorf("notification send to %s: %w", to, err)
	}

	return nil
}

// SendToUser sends a notification to a user, respecting their notification preferences.
func (s *Service) SendToUser(userID uint, subject, templateName string, data map[string]any) error {
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

	if !settings.EmailNotifications {
		logger.Debug("notification service: email notifications disabled for user", "user_id", userID)
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

func (s *Service) systemServer() *models.SMTPServer {
	return &models.SMTPServer{
		Host:       s.smtpCfg.Host,
		Port:       s.smtpCfg.Port,
		Username:   s.smtpCfg.Username,
		Password:   s.smtpCfg.Password,
		Encryption: s.smtpCfg.Encryption,
	}
}
