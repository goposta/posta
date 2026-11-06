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

package chat

import (
	"math/rand"
	"testing"
)

// stream simulates a database replay plus live events that pub/sub may drop, duplicate, or deliver before the replay.
type stream struct {
	db     []int64
	cursor Cursor
	sent   []int64
}

func (s *stream) replay() {
	for _, seq := range s.db {
		if seq > s.cursor.Last {
			s.cursor.Advance(seq)
			s.sent = append(s.sent, seq)
		}
	}
}

func (s *stream) live(seq int64) {
	switch s.cursor.Admit(seq) {
	case Deliver:
		if seq != 0 {
			s.sent = append(s.sent, seq)
		}
	case Refetch:
		s.replay()
	}
}

func assertExactlyOnceInOrder(t *testing.T, sent []int64, from, to int64) {
	t.Helper()
	if int64(len(sent)) != to-from {
		t.Fatalf("sent %d messages, want %d: %v", len(sent), to-from, sent)
	}
	for i, seq := range sent {
		if want := from + 1 + int64(i); seq != want {
			t.Fatalf("sent[%d] = %d, want %d (all: %v)", i, seq, want, sent)
		}
	}
}

func TestCursorSkipsWhatTheReplaySent(t *testing.T) {
	s := &stream{db: []int64{1, 2, 3}}
	s.replay()
	// Pub/sub delivers 2 and 3 late, after the replay already had them.
	s.live(2)
	s.live(3)
	s.db = append(s.db, 4)
	s.live(4)
	assertExactlyOnceInOrder(t, s.sent, 0, 4)
}

func TestCursorRefetchesAfterADroppedEvent(t *testing.T) {
	s := &stream{db: []int64{1, 2, 3, 4, 5}}
	s.cursor.Last = 1 // reconnected with Last-Event-ID: 1
	s.live(2)
	// 3 and 4 were dropped; 5 arrives and reveals the gap.
	s.live(5)
	assertExactlyOnceInOrder(t, s.sent, 1, 5)
	// A late copy of 4 changes nothing.
	s.live(4)
	assertExactlyOnceInOrder(t, s.sent, 1, 5)
}

func TestCursorPassesEphemeralEvents(t *testing.T) {
	c := Cursor{Last: 7}
	if got := c.Admit(0); got != Deliver {
		t.Fatalf("Admit(0) = %v, want Deliver", got)
	}
	if c.Last != 7 {
		t.Fatalf("an ephemeral event moved the cursor to %d", c.Last)
	}
}

func TestCursorAdvanceNeverGoesBack(t *testing.T) {
	c := Cursor{Last: 9}
	c.Advance(4)
	if c.Last != 9 {
		t.Fatalf("Last = %d, want 9", c.Last)
	}
}

// Whatever pub/sub does to the live events, the stream sends every message exactly once, in order.
func TestCursorExactlyOnceUnderRandomDelivery(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for run := 0; run < 500; run++ {
		const n = 40
		s := &stream{}
		resume := int64(rng.Intn(5))
		s.cursor.Last = resume
		// Messages stored before the stream connected.
		pre := int64(rng.Intn(10))
		for seq := int64(1); seq <= pre; seq++ {
			s.db = append(s.db, seq)
		}
		if pre < resume {
			pre = resume
			s.db = s.db[:0]
			for seq := int64(1); seq <= pre; seq++ {
				s.db = append(s.db, seq)
			}
		}
		// The stream subscribed before the replay, so it can still receive events for messages stored just before it.
		var inflight []int64
		for seq := pre - 2; seq <= pre; seq++ {
			if seq > 0 && rng.Intn(2) == 0 {
				inflight = append(inflight, seq)
			}
		}
		s.replay()
		for _, seq := range inflight {
			s.live(seq)
		}
		for seq := pre + 1; seq <= n; seq++ {
			s.db = append(s.db, seq)
			switch rng.Intn(6) {
			case 0: // dropped
			case 1: // duplicated
				s.live(seq)
				s.live(seq)
			case 2: // an ephemeral event slips in
				s.live(0)
				s.live(seq)
			default:
				s.live(seq)
			}
		}
		// The stream may end on a gap it has not seen yet; simulate the next message's event, which closes it.
		s.db = append(s.db, n+1)
		s.live(n + 1)
		assertExactlyOnceInOrder(t, s.sent, resume, n+1)
	}
}
