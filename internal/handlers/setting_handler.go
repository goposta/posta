// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package handlers

import (
	"strings"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/audit"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
)

func isReservedSettingKey(key string) bool {
	return strings.HasPrefix(key, "app.")
}

// visibleSettings drops reserved internal rows from a settings list.
func visibleSettings(settings []models.Setting) []models.Setting {
	out := make([]models.Setting, 0, len(settings))
	for _, s := range settings {
		if isReservedSettingKey(s.Key) {
			continue
		}
		out = append(out, s)
	}
	return out
}

// SettingHandler handles admin management of platform settings.
type SettingHandler struct {
	repo  *repositories.SettingRepository
	audit *audit.Logger
}

func NewSettingHandler(repo *repositories.SettingRepository, audit *audit.Logger) *SettingHandler {
	return &SettingHandler{repo: repo, audit: audit}
}

type UpdateSettingsRequest struct {
	Body struct {
		Settings []SettingInput `json:"settings" required:"true"`
	} `json:"body"`
}

type SettingInput struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Type  string `json:"type"`
}

func (h *SettingHandler) GetSettings(c *okapi.Context) error {
	settings, err := h.repo.FindAll()
	if err != nil {
		return c.AbortInternalServerError("failed to load settings", err)
	}
	return ok(c, visibleSettings(settings))
}

func (h *SettingHandler) UpdateSettings(c *okapi.Context, req *UpdateSettingsRequest) error {
	userID := uint(c.GetInt("user_id"))

	settings := make([]models.Setting, 0, len(req.Body.Settings))
	for _, s := range req.Body.Settings {

		if isReservedSettingKey(s.Key) {
			continue
		}
		typ := s.Type
		if typ == "" {
			typ = "string"
		}
		settings = append(settings, models.Setting{
			Key:   s.Key,
			Value: s.Value,
			Type:  typ,
		})
	}

	if err := h.repo.BulkUpsert(settings); err != nil {
		return c.AbortInternalServerError("failed to update settings", err)
	}

	h.audit.Log(userID, c.GetString("email"), c.RealIP(), "settings.updated", "Platform settings updated", nil)

	updated, err := h.repo.FindAll()
	if err != nil {
		return c.AbortInternalServerError("failed to load settings", err)
	}
	return ok(c, visibleSettings(updated))
}
