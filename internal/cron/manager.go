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

package cron

import (
	"context"
	"sync"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jkaninda/logger"
	"github.com/robfig/cron/v3"
)

type JobStatus struct {
	Name      string     `json:"name"`
	Schedule  string     `json:"schedule"`
	Running   bool       `json:"running"`
	LastRunAt *time.Time `json:"last_run_at"`
	LastError string     `json:"last_error,omitempty"`
	NextRunAt *time.Time `json:"next_run_at"`
}

type Manager struct {
	client   *asynq.Client
	jobs     []CronJob
	cron     *cron.Cron
	mu       sync.Mutex
	running  bool
	statuses map[string]*jobState
}

type jobState struct {
	running   bool
	lastRunAt *time.Time
	lastError string
}

func NewManager(client *asynq.Client) *Manager {
	return &Manager{
		client:   client,
		cron:     cron.New(cron.WithLocation(time.UTC)),
		statuses: make(map[string]*jobState),
	}
}

func (m *Manager) Register(job CronJob) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs = append(m.jobs, job)
	m.statuses[job.Name()] = &jobState{}
}

func (m *Manager) Start(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return
	}

	for _, j := range m.jobs {
		job := j // capture for closure
		_, err := m.cron.AddFunc(job.Schedule(), func() {
			m.setRunning(job.Name(), true)
			defer m.setRunning(job.Name(), false)

			now := time.Now().UTC()
			if err := job.Run(ctx, m.client); err != nil {
				m.recordRun(job.Name(), now, err.Error())
				logger.Error("cron job failed", "name", job.Name(), "error", err)
			} else {
				m.recordRun(job.Name(), now, "")
				logger.Debug("cron job completed", "name", job.Name())
			}
		})
		if err != nil {
			logger.Error("failed to register cron job", "name", job.Name(), "schedule", job.Schedule(), "error", err)
			continue
		}
		logger.Info("registered cron job", "name", job.Name(), "schedule", job.Schedule())
	}

	m.cron.Start()
	m.running = true
	logger.Info("cron manager started", "jobs", len(m.jobs))
}

func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return
	}
	m.cron.Stop()
	m.running = false
	logger.Info("cron manager stopped")
}

func (m *Manager) Jobs() []JobStatus {
	m.mu.Lock()
	defer m.mu.Unlock()

	entries := m.cron.Entries()

	result := make([]JobStatus, 0, len(m.jobs))
	for i, job := range m.jobs {
		st := m.statuses[job.Name()]
		js := JobStatus{
			Name:      job.Name(),
			Schedule:  job.Schedule(),
			Running:   st.running,
			LastRunAt: st.lastRunAt,
			LastError: st.lastError,
		}
		if m.running && i < len(entries) {
			next := entries[i].Next
			if !next.IsZero() {
				js.NextRunAt = &next
			}
		}
		result = append(result, js)
	}
	return result
}

func (m *Manager) setRunning(name string, running bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if st, ok := m.statuses[name]; ok {
		st.running = running
	}
}

func (m *Manager) recordRun(name string, at time.Time, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if st, ok := m.statuses[name]; ok {
		st.lastRunAt = &at
		st.lastError = errMsg
	}
}
