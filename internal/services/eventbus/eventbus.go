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

package eventbus

import (
	"encoding/json"
	"sync"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/logger"
)

type EventBus struct {
	repo        *repositories.EventRepository
	mu          sync.RWMutex
	subscribers map[uint64]chan *models.Event
	nextID      uint64
}

func New(repo *repositories.EventRepository) *EventBus {
	return &EventBus{
		repo:        repo,
		subscribers: make(map[uint64]chan *models.Event),
	}
}

func (b *EventBus) Publish(event *models.Event) {
	if err := b.repo.Create(event); err != nil {
		logger.Error("failed to persist event", "error", err)
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, ch := range b.subscribers {
		select {
		case ch <- event:
		default:
			// subscriber too slow, skip
		}
	}
}

func (b *EventBus) PublishSimple(category models.EventCategory, eventType string, actorID *uint, actorName, clientIP, message string, meta map[string]any) {
	b.PublishScoped(nil, category, eventType, actorID, actorName, clientIP, message, meta)
}

func (b *EventBus) PublishScoped(workspaceID *uint, category models.EventCategory, eventType string, actorID *uint, actorName, clientIP, message string, meta map[string]any) {
	var metaStr string
	if meta != nil {
		if data, err := json.Marshal(meta); err == nil {
			metaStr = string(data)
		}
	}

	b.Publish(&models.Event{
		Category:    category,
		Type:        eventType,
		WorkspaceID: workspaceID,
		ActorID:     actorID,
		ActorName:   actorName,
		ClientIP:    clientIP,
		Message:     message,
		Metadata:    metaStr,
	})
}

func (b *EventBus) Subscribe() (<-chan *models.Event, func()) {
	ch := make(chan *models.Event, 64)

	b.mu.Lock()
	id := b.nextID
	b.nextID++
	b.subscribers[id] = ch
	b.mu.Unlock()

	unsub := func() {
		b.mu.Lock()
		delete(b.subscribers, id)
		b.mu.Unlock()
		close(ch)
	}

	return ch, unsub
}
