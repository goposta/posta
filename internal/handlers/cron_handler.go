// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package handlers

import (
	"github.com/goposta/posta/internal/cron"
	"github.com/jkaninda/okapi"
)

// CronHandler exposes cron job status to admin users.
type CronHandler struct {
	manager *cron.Manager
}

func NewCronHandler(manager *cron.Manager) *CronHandler {
	return &CronHandler{manager: manager}
}

func (h *CronHandler) List(c *okapi.Context) error {
	jobs := h.manager.Jobs()
	return ok(c, jobs)
}
