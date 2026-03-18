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
	"github.com/jkaninda/okapi"
	"github.com/jkaninda/posta/internal/models"
	"github.com/jkaninda/posta/internal/services/audit"
	"github.com/jkaninda/posta/internal/storage/repositories"
)

// SettingHandler handles admin management of platform settings.
type SettingHandler struct {
	repo  *repositories.SettingRepository
	audit *audit.Logger
}

func NewSettingHandler(repo *repositories.SettingRepository, audit *audit.Logger) *SettingHandler {
	return &SettingHandler{repo: repo, audit: audit}
}

// --- Request types ---

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

// --- Handlers ---

func (h *SettingHandler) GetSettings(c *okapi.Context) error {
	settings, err := h.repo.FindAll()
	if err != nil {
		return c.AbortInternalServerError("failed to load settings", err)
	}
	return ok(c, settings)
}

func (h *SettingHandler) UpdateSettings(c *okapi.Context, req *UpdateSettingsRequest) error {
	userID := uint(c.GetInt("user_id"))

	settings := make([]models.Setting, 0, len(req.Body.Settings))
	for _, s := range req.Body.Settings {
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
	return ok(c, updated)
}
