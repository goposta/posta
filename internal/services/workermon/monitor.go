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

package workermon

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jkaninda/logger"
	"github.com/jkaninda/posta/internal/models"
	"github.com/jkaninda/posta/internal/services/eventbus"
)

// Monitor periodically checks asynq worker connections and publishes
// worker.connected / worker.disconnected events via the event bus.
type Monitor struct {
	inspector *asynq.Inspector
	bus       *eventbus.EventBus
	interval  time.Duration

	mu    sync.Mutex
	known map[string]workerInfo // keyed by "host:pid"
}

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

	// Do an initial check right away.
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
}
