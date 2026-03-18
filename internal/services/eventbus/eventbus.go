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

package eventbus

import (
	"encoding/json"
	"sync"

	"github.com/jkaninda/logger"
	"github.com/jkaninda/posta/internal/models"
	"github.com/jkaninda/posta/internal/storage/repositories"
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

// PublishSimple is a convenience method for emitting events with JSON metadata.
func (b *EventBus) PublishSimple(category models.EventCategory, eventType string, actorID *uint, actorName, clientIP, message string, meta map[string]any) {
	var metaStr string
	if meta != nil {
		if data, err := json.Marshal(meta); err == nil {
			metaStr = string(data)
		}
	}

	b.Publish(&models.Event{
		Category:  category,
		Type:      eventType,
		ActorID:   actorID,
		ActorName: actorName,
		ClientIP:  clientIP,
		Message:   message,
		Metadata:  metaStr,
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
