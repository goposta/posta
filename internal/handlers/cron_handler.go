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
	"github.com/jkaninda/okapi"
	"github.com/jkaninda/posta/internal/cron"
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
