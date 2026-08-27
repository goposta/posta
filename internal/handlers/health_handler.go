// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

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
