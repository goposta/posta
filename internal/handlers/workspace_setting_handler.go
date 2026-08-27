// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package handlers

import (
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
)

type WorkspaceSettingHandler struct {
	repo *repositories.WorkspaceSettingRepository
}

func NewWorkspaceSettingHandler(repo *repositories.WorkspaceSettingRepository) *WorkspaceSettingHandler {
	return &WorkspaceSettingHandler{repo: repo}
}

type UpdateWorkspaceSettingsRequest struct {
	Body struct {
		Timezone              *string `json:"timezone"`
		DefaultSenderName     *string `json:"default_sender_name"`
		DefaultSenderEmail    *string `json:"default_sender_email"`
		WebhookRetryCount     *int    `json:"webhook_retry_count"`
		APIKeyExpiryDays      *int    `json:"api_key_expiry_days"`
		BounceAutoSuppress    *bool   `json:"bounce_auto_suppress"`
		RequireVerifiedDomain *bool   `json:"require_verified_domain"`
	} `json:"body"`
}

// GetSettings returns the current workspace's operational settings. Any member
// may read them.
func (h *WorkspaceSettingHandler) GetSettings(c *okapi.Context) error {
	wsID := uint(c.GetInt("workspace_id"))

	settings, err := h.repo.FindByWorkspaceID(wsID)
	if err != nil {
		return c.AbortInternalServerError("failed to load settings", err)
	}
	return ok(c, settings)
}

// UpdateSettings updates the current workspace's operational settings. Restricted
// to admin/owner via the RequireWorkspaceRole middleware on the route.
func (h *WorkspaceSettingHandler) UpdateSettings(c *okapi.Context, req *UpdateWorkspaceSettingsRequest) error {
	wsID := uint(c.GetInt("workspace_id"))

	settings, err := h.repo.FindByWorkspaceID(wsID)
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
	if req.Body.WebhookRetryCount != nil {
		settings.WebhookRetryCount = *req.Body.WebhookRetryCount
	}
	if req.Body.APIKeyExpiryDays != nil {
		settings.APIKeyExpiryDays = *req.Body.APIKeyExpiryDays
	}
	if req.Body.BounceAutoSuppress != nil {
		settings.BounceAutoSuppress = *req.Body.BounceAutoSuppress
	}
	if req.Body.RequireVerifiedDomain != nil {
		settings.RequireVerifiedDomain = *req.Body.RequireVerifiedDomain
	}

	if err := h.repo.CreateOrUpdate(settings); err != nil {
		return c.AbortInternalServerError("failed to update settings", err)
	}

	return ok(c, settings)
}
