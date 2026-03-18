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

	"github.com/jkaninda/okapi"
	"github.com/jkaninda/posta/internal/services/audit"
	"github.com/jkaninda/posta/internal/services/auth"
	"github.com/jkaninda/posta/internal/storage/repositories"
)

type APIKeyHandler struct {
	service         *auth.APIKeyService
	repo            *repositories.APIKeyRepository
	userSettingRepo *repositories.UserSettingRepository
	audit           *audit.Logger
}
type CreateAPIKeyRequest struct {
	Body struct {
		Name          string   `json:"name" required:"true"`
		AllowedIPs    []string `json:"allowed_ips"`
		ExpiresInDays *int     `json:"expires_in_days"`
	} `json:"body"`
}
type RevokeAPIKeyRequest struct {
	ID int `param:"id"`
}
type DeleteAPIKeyRequest struct {
	ID int `param:"id"`
}

func NewAPIKeyHandler(service *auth.APIKeyService, repo *repositories.APIKeyRepository, userSettingRepo *repositories.UserSettingRepository, audit *audit.Logger) *APIKeyHandler {
	return &APIKeyHandler{
		service:         service,
		repo:            repo,
		userSettingRepo: userSettingRepo,
		audit:           audit,
	}
}

func (h *APIKeyHandler) Create(c *okapi.Context, req *CreateAPIKeyRequest) error {
	userID := c.GetInt("user_id")

	// Determine expiration: use provided value, fall back to user setting default
	var expiresAt *time.Time
	if req.Body.ExpiresInDays != nil {
		days := *req.Body.ExpiresInDays
		if days > 0 {
			t := time.Now().AddDate(0, 0, days)
			expiresAt = &t
		}
		// days == 0 means never expires (expiresAt stays nil)
	} else {
		// Use default from user settings
		setting, err := h.userSettingRepo.FindByUserID(uint(userID))
		if err == nil && setting.APIKeyExpiryDays > 0 {
			t := time.Now().AddDate(0, 0, setting.APIKeyExpiryDays)
			expiresAt = &t
		}
	}

	rawKey, key, err := h.service.GenerateKey(uint(userID), req.Body.Name, req.Body.AllowedIPs, expiresAt)
	if err != nil {
		return c.AbortInternalServerError("failed to create API key", err)
	}

	h.audit.Log(uint(userID), c.GetString("email"), c.RealIP(), "apikey.created", "API key created: "+req.Body.Name, nil)

	return created(c, okapi.M{
		"key":        rawKey,
		"id":         key.ID,
		"name":       key.Name,
		"prefix":     key.KeyPrefix,
		"expires_at": key.ExpiresAt,
		"message":    "Save this key securely. It will not be shown again.",
	})
}

func (h *APIKeyHandler) List(c *okapi.Context, req *ListRequest) error {
	userID := c.GetInt("user_id")
	page, size, offset := normalizePageParams(req.Page, req.Size)

	keys, total, err := h.repo.FindByUserID(uint(userID), size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list API keys")
	}

	return paginated(c, keys, total, page, size)
}

func (h *APIKeyHandler) Revoke(c *okapi.Context, req *RevokeAPIKeyRequest) error {
	userID := c.GetInt("user_id")

	key, err := h.repo.FindByID(uint(req.ID))
	if err != nil {
		return c.AbortNotFound("API key not found")
	}
	if key.UserID != uint(userID) {
		return c.AbortNotFound("API key not found")
	}

	if err := h.repo.Revoke(key.ID); err != nil {
		return c.AbortInternalServerError("failed to revoke API key")
	}

	h.audit.Log(uint(userID), c.GetString("email"), c.RealIP(), "apikey.revoked", "API key revoked: "+key.Name, nil)

	return ok(c, okapi.M{"message": "API key revoked"})
}

func (h *APIKeyHandler) Delete(c *okapi.Context, req *DeleteAPIKeyRequest) error {
	userID := c.GetInt("user_id")

	key, err := h.repo.FindByID(uint(req.ID))
	if err != nil {
		return c.AbortNotFound("API key not found")
	}
	if key.UserID != uint(userID) {
		return c.AbortNotFound("API key not found")
	}

	// Only allow deletion of expired or revoked keys
	if key.IsValid() {
		return c.AbortBadRequest("active API keys cannot be deleted — revoke it first")
	}

	if err := h.repo.Delete(key.ID); err != nil {
		return c.AbortInternalServerError("failed to delete API key")
	}

	h.audit.Log(uint(userID), c.GetString("email"), c.RealIP(), "apikey.deleted", "API key deleted: "+key.Name, nil)

	return ok(c, okapi.M{"message": "API key deleted"})
}
