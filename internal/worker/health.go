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

package worker

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/goposta/posta/internal/health"
	"github.com/goposta/posta/internal/metrics"
	"github.com/jkaninda/logger"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// HealthServer gives a dedicated worker, which has no HTTP surface of its own, probes and a metrics endpoint.
type HealthServer struct {
	db    *gorm.DB
	redis *redis.Client
	srv   *http.Server

	// Reachable dependencies are not enough: a worker that stopped consuming would report ready as the queue backs up.
	processing atomic.Bool
}

// WorkerStatus spells out health.Status's fields rather than embedding it, avoiding status.Status.Status.
type WorkerStatus struct {
	Status     string `json:"status"`
	Database   string `json:"database"`
	Redis      string `json:"redis"`
	Processing bool   `json:"processing"`
}

func NewHealthServer(addr string, db *gorm.DB, rdb *redis.Client) *HealthServer {
	h := &HealthServer{db: db, redis: rdb}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", h.healthz)
	mux.HandleFunc("/readyz", h.readyz)
	mux.Handle("/metrics", metrics.HTTPHandler())

	h.srv = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return h
}

func (h *HealthServer) SetProcessing(running bool) { h.processing.Store(running) }

// A bind failure is logged, not fatal: an occupied probe port must not take down a healthy mail worker.
func (h *HealthServer) Start() func(context.Context) {
	go func() {
		if err := h.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("worker health server stopped; probes and metrics are unavailable",
				"addr", h.srv.Addr, "error", err)
		}
	}()

	return func(ctx context.Context) {
		if err := h.srv.Shutdown(ctx); err != nil {
			logger.Warn("worker health server did not shut down cleanly", "error", err)
		}
	}
}

// healthz touches nothing else: failing liveness on a DB blip restarts the worker and loses in-flight work.
func (h *HealthServer) healthz(w http.ResponseWriter, _ *http.Request) {
	// Only status: empty database/redis entries would read as failed checks and differ from the server's /healthz.
	writeJSON(w, http.StatusOK, map[string]string{"status": health.StatusOK})
}

func (h *HealthServer) readyz(w http.ResponseWriter, r *http.Request) {
	deps := health.Check(r.Context(), h.db, h.redis)
	processing := h.processing.Load()

	status := WorkerStatus{
		Status:     deps.Status,
		Database:   deps.Database,
		Redis:      deps.Redis,
		Processing: processing,
	}
	if !processing {
		status.Status = health.StatusNotReady
	}

	code := http.StatusOK
	if status.Status != health.StatusReady {
		code = http.StatusServiceUnavailable
	}
	writeJSON(w, code, status)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
