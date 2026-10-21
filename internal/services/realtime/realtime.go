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

// Package realtime fans out live events to SSE subscribers across every Posta
// process. Unlike eventbus, it does not persist anything and it is not limited
// to the process that published: the Redis implementation lets the worker
// publish an event that a dashboard connected to any server replica receives.
//
// Delivery is best effort. A subscriber that is not connected when an event is
// published never sees it, so the database stays the source of truth and an
// event is only a hint to refetch.
package realtime

import (
	"context"
	"fmt"
	"sync"
)

// Event is one live notification.
type Event struct {
	Type        string `json:"type"`
	WorkspaceID uint   `json:"workspace_id"`
	Topic       string `json:"topic"`
	Data        any    `json:"data,omitempty"`
}

// Bus publishes and subscribes to topics.
type Bus interface {
	// Publish sends ev to every current subscriber of ev.Topic, in every process.
	Publish(ctx context.Context, ev Event) error
	// Subscribe returns a channel of events on topic and a function that ends
	// the subscription and closes the channel. The function is safe to call once.
	Subscribe(ctx context.Context, topic string) (<-chan Event, func())
}

// WorkspaceTopic is the topic that carries a workspace's dashboard events.
// Topics are built from database IDs only, never from client input.
func WorkspaceTopic(workspaceID uint) string {
	return fmt.Sprintf("ws:%d", workspaceID)
}

// subscriberBuffer is how many undelivered events a subscriber may hold before
// further events are dropped for it.
const subscriberBuffer = 64

// hub is the in-process fan-out shared by both Bus implementations.
type hub struct {
	mu     sync.RWMutex
	topics map[string]map[uint64]chan Event
	nextID uint64
}

func newHub() *hub {
	return &hub{topics: make(map[string]map[uint64]chan Event)}
}

// add registers a subscriber and reports whether it is the topic's first.
func (h *hub) add(topic string) (uint64, chan Event, bool) {
	ch := make(chan Event, subscriberBuffer)
	h.mu.Lock()
	defer h.mu.Unlock()
	subs, ok := h.topics[topic]
	if !ok {
		subs = make(map[uint64]chan Event)
		h.topics[topic] = subs
	}
	id := h.nextID
	h.nextID++
	subs[id] = ch
	return id, ch, !ok
}

// remove drops a subscriber and reports whether the topic has none left.
func (h *hub) remove(topic string, id uint64) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	subs, ok := h.topics[topic]
	if !ok {
		return false
	}
	if ch, ok := subs[id]; ok {
		delete(subs, id)
		close(ch)
	}
	if len(subs) == 0 {
		delete(h.topics, topic)
		return true
	}
	return false
}

// deliver hands ev to every local subscriber of its topic. A subscriber whose
// buffer is full misses the event rather than stalling everyone else.
func (h *hub) deliver(ev Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, ch := range h.topics[ev.Topic] {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (h *hub) topicCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.topics)
}

// memoryBus is a single-process Bus, for tests and for deployments that run
// the worker embedded in the server.
type memoryBus struct {
	hub *hub
}

// NewMemory returns a Bus that only reaches subscribers in this process.
func NewMemory() Bus {
	return &memoryBus{hub: newHub()}
}

func (b *memoryBus) Publish(_ context.Context, ev Event) error {
	b.hub.deliver(ev)
	return nil
}

func (b *memoryBus) Subscribe(_ context.Context, topic string) (<-chan Event, func()) {
	id, ch, _ := b.hub.add(topic)
	var once sync.Once
	return ch, func() { once.Do(func() { b.hub.remove(topic, id) }) }
}
