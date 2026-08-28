// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package handlers

import (
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
)

// UserSettingHandler handles per-user settings.
type UserSettingHandler struct {
	repo *repositories.UserSettingRepository
}

func NewUserSettingHandler(repo *repositories.UserSettingRepository) *UserSettingHandler {
	return &UserSettingHandler{repo: repo}
}

type UpdateUserSettingsRequest struct {
	Body struct {
		Timezone           *string `json:"timezone"`
		DefaultSenderName  *string `json:"default_sender_name"`
		DefaultSenderEmail *string `json:"default_sender_email"`
		EmailNotifications *bool   `json:"email_notifications"`
		NotificationEmail  *string `json:"notification_email"`
		WebhookRetryCount  *int    `json:"webhook_retry_count"`
		DefaultTemplateID  *uint   `json:"default_template_id"`
		APIKeyExpiryDays   *int    `json:"api_key_expiry_days"`
		BounceAutoSuppress *bool   `json:"bounce_auto_suppress"`
		DefaultLanguage    *string `json:"default_language"`
		DailyReport        *bool   `json:"daily_report"`

		NotifyBounceAlerts      *bool `json:"notify_bounce_alerts"`
		NotifyAPIKeyExpiry      *bool `json:"notify_api_key_expiry"`
		NotifyWorkspaceActivity *bool `json:"notify_workspace_activity"`
		NotifyNewMessage        *bool `json:"notify_new_message"`
	} `json:"body"`
}

func (h *UserSettingHandler) GetSettings(c *okapi.Context) error {
	userID := uint(c.GetInt("user_id"))

	settings, err := h.repo.FindByUserID(userID)
	if err != nil {
		return c.AbortInternalServerError("failed to load settings", err)
	}
	return ok(c, settings)
}

func (h *UserSettingHandler) UpdateSettings(c *okapi.Context, req *UpdateUserSettingsRequest) error {
	userID := uint(c.GetInt("user_id"))

	settings, err := h.repo.FindByUserID(userID)
	if err != nil {
		return c.AbortInternalServerError("failed to load settings", err)
	}

	if req.Body.Timezone != nil {
		settings.Timezone = *req.Body.Timezone
	}
	if req.Body.DefaultSenderName != nil {
		settings.DefaultSenderName = *req.Body.DefaultSenderName
	}
	if req.Body.DefaultSenderEmail != nil {
		settings.DefaultSenderEmail = *req.Body.DefaultSenderEmail
	}
	if req.Body.EmailNotifications != nil {
		settings.EmailNotifications = *req.Body.EmailNotifications
	}
	if req.Body.NotificationEmail != nil {
		settings.NotificationEmail = *req.Body.NotificationEmail
	}
	if req.Body.WebhookRetryCount != nil {
		settings.WebhookRetryCount = *req.Body.WebhookRetryCount
	}
	if req.Body.DefaultTemplateID != nil {
		settings.DefaultTemplateID = req.Body.DefaultTemplateID
	}
	if req.Body.APIKeyExpiryDays != nil {
		settings.APIKeyExpiryDays = *req.Body.APIKeyExpiryDays
	}
	if req.Body.BounceAutoSuppress != nil {
		settings.BounceAutoSuppress = *req.Body.BounceAutoSuppress
	}
	if req.Body.DefaultLanguage != nil {
		settings.DefaultLanguage = *req.Body.DefaultLanguage
	}
	if req.Body.DailyReport != nil {
		settings.DailyReport = *req.Body.DailyReport
	}
	if req.Body.NotifyBounceAlerts != nil {
		settings.NotifyBounceAlerts = *req.Body.NotifyBounceAlerts
	}
	if req.Body.NotifyAPIKeyExpiry != nil {
		settings.NotifyAPIKeyExpiry = *req.Body.NotifyAPIKeyExpiry
	}
	if req.Body.NotifyNewMessage != nil {
		settings.NotifyNewMessage = *req.Body.NotifyNewMessage
	}
	if req.Body.NotifyWorkspaceActivity != nil {
		settings.NotifyWorkspaceActivity = *req.Body.NotifyWorkspaceActivity
	}

	if err := h.repo.CreateOrUpdate(settings); err != nil {
		return c.AbortInternalServerError("failed to update settings", err)
	}

	return ok(c, settings)
}
