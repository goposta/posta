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
	"context"
	"time"

	"github.com/jkaninda/okapi"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const statusNotReady = "not ready"

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

type ReadyResponse struct {
	Status   string `json:"status" example:"ready"`
	Database string `json:"database" example:"ok"`
	Redis    string `json:"redis" example:"ok"`
}

// Healthz is a lightweight liveness probe.
func (h *HealthHandler) Healthz(c *okapi.Context) error {
	return c.OK(HealthResponse{Status: "ok"})
}

// Readyz checks that all dependencies are reachable.
func (h *HealthHandler) Readyz(c *okapi.Context) error {
	resp := ReadyResponse{
		Status:   "ready",
		Database: "ok",
		Redis:    "ok",
	}

	sqlDB, err := h.db.DB()
	if err != nil {
		resp.Status = statusNotReady
		resp.Database = err.Error()
	} else {
		ctx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Second)
		defer cancel()
		if err := sqlDB.PingContext(ctx); err != nil {
			resp.Status = statusNotReady
			resp.Database = err.Error()
		}
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Second)
	defer cancel()
	if err := h.redis.Ping(ctx).Err(); err != nil {
		resp.Status = statusNotReady
		resp.Redis = err.Error()
	}

	if resp.Status != "ready" {
		return c.JSON(503, resp)
	}
	return c.OK(resp)
}
