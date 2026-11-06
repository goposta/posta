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
	"time"

	"github.com/goposta/posta/internal/enterprise"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/eventbus"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
)

type EventHandler struct {
	repo    *repositories.EventRepository
	bus     *eventbus.EventBus
	edition enterprise.EE
}

const CommunityAuditWindow = 24 * time.Hour

// SetEdition decides how much of the workspace audit trail is shown.
func (h *EventHandler) SetEdition(ee enterprise.EE) { h.edition = ee }

func (h *EventHandler) licensed() bool {
	return h.edition != nil && h.edition.Licensed()
}

// communityAuditSince is the oldest audit event Posta Community shows.
func communityAuditSince() time.Time {
	return time.Now().Add(-CommunityAuditWindow)
}

func isWorkspaceAudit(e *models.Event) bool {
	return e.Category == models.EventCategoryAudit && e.WorkspaceID != nil
}

type ListEventsRequest struct {
	Page     int    `query:"page" default:"0"`
	Size     int    `query:"size" default:"20"`
	Category string `query:"category"`
	Search   string `query:"search"`
}

func NewEventHandler(repo *repositories.EventRepository, bus *eventbus.EventBus) *EventHandler {
	return &EventHandler{repo: repo, bus: bus}
}

type EventRequest struct {
	ID int `param:"id"`
}

// Get returns a single event by ID (admin scope, any category or workspace).
func (h *EventHandler) Get(c *okapi.Context, req *EventRequest) error {
	event, err := h.repo.FindByID(uint(req.ID))
	if err != nil || (isWorkspaceAudit(event) && !h.licensed()) {
		return c.AbortNotFound("event not found")
	}
	return ok(c, event)
}

func (h *EventHandler) List(c *okapi.Context, req *ListEventsRequest) error {
	page, size, offset := normalizePageParams(req.Page, req.Size)

	var events []models.Event
	var total int64
	var err error

	if req.Category != "" {
		events, total, err = h.repo.FindByCategory(models.EventCategory(req.Category), req.Search, !h.licensed(), size, offset)
	} else {
		events, total, err = h.repo.FindAll(req.Search, !h.licensed(), size, offset)
	}
	if err != nil {
		return c.AbortInternalServerError("failed to list events")
	}

	return paginated(c, events, total, page, size)
}

func (h *EventHandler) UserAuditLog(c *okapi.Context, req *ListEventsRequest) error {
	userID := c.GetInt("user_id")
	page, size, offset := normalizePageParams(req.Page, req.Size)

	events, total, err := h.repo.FindPersonalByActorAndCategory(uint(userID), models.EventCategoryAudit, size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list audit events")
	}

	return paginated(c, events, total, page, size)
}

func (h *EventHandler) WorkspaceAuditLog(c *okapi.Context, req *ListEventsRequest) error {
	wsID := c.GetInt("workspace_id")
	page, size, offset := normalizePageParams(req.Page, req.Size)

	var since time.Time
	if !h.licensed() {
		since = communityAuditSince()
	}
	events, total, err := h.repo.FindByWorkspaceAndCategory(uint(wsID), models.EventCategoryAudit, since, size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list audit events")
	}

	return paginated(c, events, total, page, size)
}

type AuditEventRequest struct {
	ID int `param:"id"`
}

func (h *EventHandler) WorkspaceAuditLogDetail(c *okapi.Context, req *AuditEventRequest) error {
	wsID := uint(c.GetInt("workspace_id"))

	event, err := h.repo.FindByID(uint(req.ID))
	if err != nil {
		return c.AbortNotFound("audit event not found")
	}

	if event.Category != models.EventCategoryAudit || event.WorkspaceID == nil || *event.WorkspaceID != wsID {
		return c.AbortNotFound("audit event not found")
	}
	if !h.licensed() && event.CreatedAt.Before(communityAuditSince()) {
		edition := h.edition
		if edition == nil {
			edition = enterprise.Community()
		}
		return licenseAbort(c, edition.Require())
	}

	return ok(c, event)
}

func (h *EventHandler) Stream(c *okapi.Context) error {
	ctx := c.Request().Context()

	ch, unsub := h.bus.Subscribe()
	defer unsub()

	msgCh := make(chan okapi.Message)
	go func() {
		defer close(msgCh)
		for {
			select {
			case <-ctx.Done():
				return
			case evt, ok := <-ch:
				if !ok {
					return
				}
				if isWorkspaceAudit(evt) && !h.licensed() {
					continue
				}
				select {
				case msgCh <- okapi.Message{Event: evt.Type, Data: evt}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return c.SSEStreamWithOptions(ctx, msgCh, &okapi.StreamOptions{
		Serializer:   &okapi.JSONSerializer{},
		PingInterval: 30 * time.Second,
	})
}
