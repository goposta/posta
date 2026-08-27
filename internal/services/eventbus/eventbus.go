// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package eventbus

import (
	"encoding/json"
	"sync"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/logger"
)

// EventBus provides an in-memory pub/sub for events with database persistence.
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

// Publish persists an event and broadcasts it to all SSE subscribers.
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

// Subscribe returns a channel that receives new events and a function to unsubscribe.
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
