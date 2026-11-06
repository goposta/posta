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

// Decision is what a stream does with a live event.
type Decision int

const (
	// Deliver sends the event on.
	Deliver Decision = iota
	// Skip drops an event the stream already sent, from its replay.
	Skip
	// Refetch means events were missed: the stream must read everything
	// after Cursor.Last from the database, which includes this event.
	Refetch
)

// Cursor is the newest sequence number a stream has sent. Live events reach
// a stream through pub/sub, which drops events while a subscriber is away or
// slow, and they can arrive before or after the database replay a stream
// starts with. The cursor turns that into an exactly-once, in-order stream:
// the database is the truth and pub/sub only rings the bell.
type Cursor struct {
	Last int64
}

// Admit decides what to do with a live event carrying seq. Zero is an
// ephemeral event, such as typing, which has no place in the order.
func (c *Cursor) Admit(seq int64) Decision {
	switch {
	case seq == 0:
		return Deliver
	case seq <= c.Last:
		return Skip
	case seq == c.Last+1:
		c.Last = seq
		return Deliver
	default:
		return Refetch
	}
}

// Advance records that the stream sent seq.
func (c *Cursor) Advance(seq int64) {
	if seq > c.Last {
		c.Last = seq
	}
}
