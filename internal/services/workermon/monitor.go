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

package workermon

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/eventbus"
	"github.com/hibiken/asynq"
	"github.com/jkaninda/logger"
)

// Monitor polls asynq workers and publishes worker.connected / worker.disconnected events.
type Monitor struct {
	inspector *asynq.Inspector
	bus       *eventbus.EventBus
	interval  time.Duration
	onCount   func(int)

	mu    sync.Mutex
	known map[string]workerInfo // keyed by "host:pid"
}

// OnCount's callback feeds the metrics gauges with the worker count after each poll.
func (m *Monitor) OnCount(fn func(int)) { m.onCount = fn }

type workerInfo struct {
	Host   string
	PID    int
	Queues map[string]int
}

func workerKey(host string, pid int) string {
	return fmt.Sprintf("%s:%d", host, pid)
}

func New(inspector *asynq.Inspector, bus *eventbus.EventBus, interval time.Duration) *Monitor {
	return &Monitor{
		inspector: inspector,
		bus:       bus,
		interval:  interval,
		known:     make(map[string]workerInfo),
	}
}

// Start begins polling in a background goroutine. It stops when ctx is cancelled.
func (m *Monitor) Start(ctx context.Context) {
	go m.run(ctx)
}

func (m *Monitor) run(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	m.check()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.check()
		}
	}
}

func (m *Monitor) check() {
	servers, err := m.inspector.Servers()
	if err != nil {
		logger.Error("worker monitor: failed to query servers", "error", err)
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	current := make(map[string]workerInfo, len(servers))
	for _, s := range servers {
		key := workerKey(s.Host, s.PID)
		current[key] = workerInfo{
			Host:   s.Host,
			PID:    s.PID,
			Queues: s.Queues,
		}

		if _, exists := m.known[key]; !exists {
			m.bus.PublishSimple(
				models.EventCategorySystem,
				"worker.connected",
				nil, "system", "",
				fmt.Sprintf("Worker connected on %s (PID %d)", s.Host, s.PID),
				map[string]any{
					"host":   s.Host,
					"pid":    s.PID,
					"queues": s.Queues,
				},
			)
		}
	}

	for key, info := range m.known {
		if _, exists := current[key]; !exists {
			m.bus.PublishSimple(
				models.EventCategorySystem,
				"worker.disconnected",
				nil, "system", "",
				fmt.Sprintf("Worker disconnected from %s (PID %d)", info.Host, info.PID),
				map[string]any{
					"host": info.Host,
					"pid":  info.PID,
				},
			)
		}
	}

	m.known = current

	if m.onCount != nil {
		m.onCount(len(current))
	}
}
