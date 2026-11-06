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

package handlers

import (
	"net/http"

	"github.com/goposta/posta/internal/health"
	"github.com/jkaninda/okapi"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type HealthHandler struct {
	db    *gorm.DB
	redis *redis.Client
}

func NewHealthHandler(db *gorm.DB, redis *redis.Client) *HealthHandler {
	return &HealthHandler{db: db, redis: redis}
}

type HealthResponse struct {
	Status string `json:"status" example:"ok"`
}

// ReadyResponse is shared with dedicated workers so a probe of either reads the same.
type ReadyResponse = health.Status

func (h *HealthHandler) Healthz(c *okapi.Context) error {
	return c.OK(HealthResponse{Status: health.StatusOK})
}

func (h *HealthHandler) Readyz(c *okapi.Context) error {
	status := health.Check(c.Request().Context(), h.db, h.redis)
	if !status.Ready() {
		return c.JSON(http.StatusServiceUnavailable, status)
	}
	return c.OK(status)
}
