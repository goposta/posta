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
	"time"

	"github.com/jkaninda/okapi"
	"github.com/jkaninda/posta/internal/models"
	"github.com/jkaninda/posta/internal/services/eventbus"
	"github.com/jkaninda/posta/internal/storage/repositories"
)

type EventHandler struct {
	repo *repositories.EventRepository
	bus  *eventbus.EventBus
}

type ListEventsRequest struct {
	Page     int    `query:"page" default:"0"`
	Size     int    `query:"size" default:"20"`
	Category string `query:"category"`
}

func NewEventHandler(repo *repositories.EventRepository, bus *eventbus.EventBus) *EventHandler {
	return &EventHandler{repo: repo, bus: bus}
}

// List returns paginated historical events.
func (h *EventHandler) List(c *okapi.Context, req *ListEventsRequest) error {
	page, size, offset := normalizePageParams(req.Page, req.Size)

	var events []models.Event
	var total int64
	var err error

	if req.Category != "" {
		events, total, err = h.repo.FindByCategory(models.EventCategory(req.Category), size, offset)
	} else {
		events, total, err = h.repo.FindAll(size, offset)
	}
	if err != nil {
		return c.AbortInternalServerError("failed to list events")
	}

	return paginated(c, events, total, page, size)
}

// UserAuditLog returns paginated audit events for the authenticated user.
func (h *EventHandler) UserAuditLog(c *okapi.Context, req *ListEventsRequest) error {
	userID := c.GetInt("user_id")
	page, size, offset := normalizePageParams(req.Page, req.Size)

	events, total, err := h.repo.FindByActorAndCategory(uint(userID), models.EventCategoryAudit, size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list audit events")
	}

	return paginated(c, events, total, page, size)
}

// Stream sends real-time events to the admin via SSE.
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
