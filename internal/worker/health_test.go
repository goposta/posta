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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func get(t *testing.T, h *HealthServer, path string) (*httptest.ResponseRecorder, WorkerStatus) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.srv.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

	var body WorkerStatus
	if strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s returned invalid JSON: %v (%s)", path, err, rec.Body.String())
		}
	}
	return rec, body
}

// A liveness probe failing on a database blip restarts the worker, losing in-flight work without fixing the DB.
func TestHealthzIsIndependentOfDependencies(t *testing.T) {
	h := NewHealthServer(":0", nil, nil)

	rec, body := get(t, h, "/healthz")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with no dependencies at all, got %d", rec.Code)
	}
	if body.Status != "ok" {
		t.Errorf("expected status ok, got %q", body.Status)
	}

	// Empty database and redis fields would read as failed checks rather than checks never made.
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(raw) != 1 || raw["status"] != "ok" {
		t.Errorf("liveness payload should be exactly {\"status\":\"ok\"}, got %v", raw)
	}
}

// Unreachable dependencies must not read as ready.
func TestReadyzFailsWithoutDependencies(t *testing.T) {
	h := NewHealthServer(":0", nil, nil)
	h.SetProcessing(true)

	rec, body := get(t, h, "/readyz")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when nothing is reachable, got %d", rec.Code)
	}
	if body.Status != "not ready" {
		t.Errorf("expected 'not ready', got %q", body.Status)
	}
	if body.Database == "ok" || body.Redis == "ok" {
		t.Errorf("dependencies should report why they failed, got db=%q redis=%q", body.Database, body.Redis)
	}
}

// A process that answers pings but stopped consuming is not ready: work would queue behind a healthy-looking worker.
func TestReadyzRequiresProcessing(t *testing.T) {
	h := NewHealthServer(":0", nil, nil)

	_, stopped := get(t, h, "/readyz")
	if stopped.Processing {
		t.Fatal("a worker that has not started must not report processing")
	}

	h.SetProcessing(true)
	_, running := get(t, h, "/readyz")
	if !running.Processing {
		t.Fatal("SetProcessing(true) should be reflected in the payload")
	}

	h.SetProcessing(false)
	rec, drained := get(t, h, "/readyz")
	if drained.Processing {
		t.Error("a drained worker must stop reporting processing")
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("a drained worker must not be ready, got %d", rec.Code)
	}
}

// The counters the worker increments are useless if nothing can scrape them.
func TestMetricsAreExposed(t *testing.T) {
	h := NewHealthServer(":0", nil, nil)

	rec := httptest.NewRecorder()
	h.srv.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from /metrics, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "# HELP") {
		t.Error("expected Prometheus exposition format")
	}
}

// The payload is an operator contract: probes and dashboards read these names.
func TestReadyzPayloadShape(t *testing.T) {
	h := NewHealthServer(":0", nil, nil)

	rec := httptest.NewRecorder()
	h.srv.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	for _, key := range []string{"status", "database", "redis", "processing"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("readiness payload is missing %q: %v", key, raw)
		}
	}
	// Nothing beyond the documented contract.
	if len(raw) != 4 {
		t.Errorf("unexpected fields in the payload: %v", raw)
	}
}

// Shutting down must not leave the listener holding the port.
func TestStartAndStop(t *testing.T) {
	h := NewHealthServer("127.0.0.1:0", nil, nil)
	stop := h.Start()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	stop(ctx)

	// A second shutdown is a no-op, not a panic: deferred cleanup can run twice on some exit paths.
	stop(ctx)
}
