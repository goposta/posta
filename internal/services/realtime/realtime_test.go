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
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func receive(t *testing.T, ch <-chan Event) Event {
	t.Helper()
	select {
	case ev, ok := <-ch:
		if !ok {
			t.Fatal("channel closed before an event arrived")
		}
		return ev
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for an event")
	}
	return Event{}
}

func expectNothing(t *testing.T, ch <-chan Event) {
	t.Helper()
	select {
	case ev, ok := <-ch:
		if ok {
			t.Fatalf("unexpected event %+v", ev)
		}
	case <-time.After(50 * time.Millisecond):
	}
}

func TestMemoryFanOutToEverySubscriberOfATopic(t *testing.T) {
	bus := NewMemory()
	ctx := context.Background()

	a, unsubA := bus.Subscribe(ctx, WorkspaceTopic(1))
	defer unsubA()
	b, unsubB := bus.Subscribe(ctx, WorkspaceTopic(1))
	defer unsubB()
	other, unsubOther := bus.Subscribe(ctx, WorkspaceTopic(2))
	defer unsubOther()

	if err := bus.Publish(ctx, Event{Type: "message.received", Topic: WorkspaceTopic(1)}); err != nil {
		t.Fatal(err)
	}

	if got := receive(t, a); got.Type != "message.received" {
		t.Fatalf("a got %q", got.Type)
	}
	if got := receive(t, b); got.Type != "message.received" {
		t.Fatalf("b got %q", got.Type)
	}
	expectNothing(t, other)
}

func TestMemoryUnsubscribeClosesChannelAndIsIdempotent(t *testing.T) {
	bus := NewMemory()
	ch, unsub := bus.Subscribe(context.Background(), "ws:1")
	unsub()
	unsub()
	if _, ok := <-ch; ok {
		t.Fatal("channel should be closed after unsubscribe")
	}
}

func TestHubTearsDownTopicWithLastSubscriber(t *testing.T) {
	h := newHub()
	id1, _, first := h.add("ws:1")
	if !first {
		t.Fatal("first subscriber should report first")
	}
	id2, _, first := h.add("ws:1")
	if first {
		t.Fatal("second subscriber should not report first")
	}
	if h.remove("ws:1", id1) {
		t.Fatal("topic should remain while a subscriber is left")
	}
	if !h.remove("ws:1", id2) {
		t.Fatal("removing the last subscriber should report the topic empty")
	}
	if n := h.topicCount(); n != 0 {
		t.Fatalf("topicCount = %d, want 0", n)
	}
}

func TestSlowSubscriberIsDroppedNotBlocking(t *testing.T) {
	bus := NewMemory()
	ctx := context.Background()
	slow, unsubSlow := bus.Subscribe(ctx, "ws:1")
	defer unsubSlow()
	fast, unsubFast := bus.Subscribe(ctx, "ws:1")
	defer unsubFast()

	done := make(chan struct{})
	go func() {
		for i := 0; i < subscriberBuffer*3; i++ {
			_ = bus.Publish(ctx, Event{Type: "tick", Topic: "ws:1"})
			<-fast
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("publishing blocked on a slow subscriber")
	}
	if n := len(slow); n != subscriberBuffer {
		t.Fatalf("slow subscriber holds %d events, want its buffer of %d", n, subscriberBuffer)
	}
}

// TestRedisBusAcrossClients runs only against a real Redis, set via
// POSTA_TEST_REDIS_URL (for example redis://localhost:6379/15). Two buses with
// separate clients stand in for two processes.
func TestRedisBusAcrossClients(t *testing.T) {
	url := os.Getenv("POSTA_TEST_REDIS_URL")
	if url == "" {
		t.Skip("POSTA_TEST_REDIS_URL not set")
	}
	opt, err := redis.ParseURL(url)
	if err != nil {
		t.Fatal(err)
	}
	pubClient, subClient := redis.NewClient(opt), redis.NewClient(opt)
	defer func() { _ = pubClient.Close(); _ = subClient.Close() }()

	publisher, subscriber := NewRedis(pubClient), NewRedis(subClient)
	ctx := context.Background()
	ch, unsub := subscriber.Subscribe(ctx, "ws:test")
	defer unsub()
	time.Sleep(100 * time.Millisecond) // let SUBSCRIBE reach the server

	if err := publisher.Publish(ctx, Event{Type: "message.received", WorkspaceID: 7, Topic: "ws:test"}); err != nil {
		t.Fatal(err)
	}
	got := receive(t, ch)
	if got.Type != "message.received" || got.WorkspaceID != 7 || got.Topic != "ws:test" {
		t.Fatalf("got %+v", got)
	}
}
