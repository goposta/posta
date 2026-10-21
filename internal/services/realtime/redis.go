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

package realtime

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/jkaninda/logger"
	"github.com/redis/go-redis/v9"
)

// channelPrefix namespaces Posta's pub/sub channels on a shared Redis.
const channelPrefix = "posta:rt:"

// redisBus fans out through Redis pub/sub. Each process holds one pub/sub
// connection and subscribes to a Redis channel only while it has at least one
// local subscriber for that topic, so a replica with thousands of open streams
// on one workspace still holds a single Redis subscription for it.
type redisBus struct {
	rdb *redis.Client
	hub *hub

	mu      sync.Mutex
	ps      *redis.PubSub
	started bool
}

// NewRedis returns a Bus backed by Redis pub/sub.
func NewRedis(rdb *redis.Client) Bus {
	return &redisBus{rdb: rdb, hub: newHub()}
}

func (b *redisBus) Publish(ctx context.Context, ev Event) error {
	payload, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	return b.rdb.Publish(ctx, channelPrefix+ev.Topic, payload).Err()
}

func (b *redisBus) Subscribe(ctx context.Context, topic string) (<-chan Event, func()) {
	id, ch, first := b.hub.add(topic)
	if first {
		b.redisSubscribe(ctx, topic)
	}
	var once sync.Once
	return ch, func() {
		once.Do(func() {
			if b.hub.remove(topic, id) {
				b.redisUnsubscribe(topic)
			}
		})
	}
}

// redisSubscribe adds topic to the process-wide pub/sub connection, opening it
// and starting the reader on first use.
func (b *redisBus) redisSubscribe(ctx context.Context, topic string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.ps == nil {
		b.ps = b.rdb.Subscribe(context.Background())
	}
	if err := b.ps.Subscribe(ctx, channelPrefix+topic); err != nil {
		logger.Warn("realtime: redis subscribe failed", "topic", topic, "error", err)
	}
	if !b.started {
		b.started = true
		go b.read(b.ps)
	}
}

func (b *redisBus) redisUnsubscribe(topic string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.ps == nil {
		return
	}
	if err := b.ps.Unsubscribe(context.Background(), channelPrefix+topic); err != nil {
		logger.Warn("realtime: redis unsubscribe failed", "topic", topic, "error", err)
	}
}

// read relays every Redis message to local subscribers. go-redis reconnects
// and resubscribes on its own, so this loop only ends if the PubSub is closed.
func (b *redisBus) read(ps *redis.PubSub) {
	for msg := range ps.Channel() {
		var ev Event
		if err := json.Unmarshal([]byte(msg.Payload), &ev); err != nil {
			logger.Warn("realtime: dropping malformed event", "channel", msg.Channel, "error", err)
			continue
		}
		// The channel name is authoritative; never trust a topic carried in a payload.
		ev.Topic = strings.TrimPrefix(msg.Channel, channelPrefix)
		b.hub.deliver(ev)
	}
}
