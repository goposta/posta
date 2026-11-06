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
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// Agent presence statuses.
const (
	StatusOnline  = "online"
	StatusAway    = "away"
	StatusOffline = "offline"
)

const (
	// presenceTTL is how long a heartbeat counts. The dashboard sends one
	// every 30 seconds, so a closed tab reads as offline within 75 seconds,
	// with no cleanup job.
	presenceTTL = 75 * time.Second
	// visitorLiveTTL is how long a visitor stream's mark lasts without a
	// refresh; streams refresh it on every keep-alive.
	visitorLiveTTL = 60 * time.Second
	typingThrottle = 3 * time.Second
)

// Presence tracks which agents are available, in one Redis hash per
// workspace: field user ID, value "status:unix seconds". Stale entries are
// ignored on read, so a missed heartbeat simply ages out.
type Presence struct {
	rdb *redis.Client
	now func() time.Time
}

func NewPresence(rdb *redis.Client) *Presence {
	return &Presence{rdb: rdb, now: time.Now}
}

func presenceKey(workspaceID uint) string { return fmt.Sprintf("chat:presence:%d", workspaceID) }

// Set records an agent's status and reports whether it changed.
func (p *Presence) Set(ctx context.Context, workspaceID, userID uint, status string) (bool, error) {
	if p == nil || p.rdb == nil {
		return false, nil
	}
	before, err := p.List(ctx, workspaceID)
	if err != nil {
		return false, err
	}
	key, field := presenceKey(workspaceID), strconv.FormatUint(uint64(userID), 10)
	if status == StatusOffline {
		if err := p.rdb.HDel(ctx, key, field).Err(); err != nil {
			return false, err
		}
	} else {
		pipe := p.rdb.TxPipeline()
		pipe.HSet(ctx, key, field, status+":"+strconv.FormatInt(p.now().Unix(), 10))
		pipe.Expire(ctx, key, time.Hour)
		if _, err := pipe.Exec(ctx); err != nil {
			return false, err
		}
	}
	previous := before[userID]
	if previous == "" {
		previous = StatusOffline
	}
	return previous != status, nil
}

// List returns the agents whose heartbeat is fresh, with their status.
func (p *Presence) List(ctx context.Context, workspaceID uint) (map[uint]string, error) {
	out := map[uint]string{}
	if p == nil || p.rdb == nil {
		return out, nil
	}
	fields, err := p.rdb.HGetAll(ctx, presenceKey(workspaceID)).Result()
	if err != nil {
		return out, err
	}
	return freshPresence(fields, p.now()), nil
}

// AnyOnline reports whether at least one agent is online. Away does not count.
func (p *Presence) AnyOnline(ctx context.Context, workspaceID uint) bool {
	list, err := p.List(ctx, workspaceID)
	if err != nil {
		return false
	}
	for _, status := range list {
		if status == StatusOnline {
			return true
		}
	}
	return false
}

func freshPresence(fields map[string]string, now time.Time) map[uint]string {
	out := make(map[uint]string, len(fields))
	for field, value := range fields {
		id, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			continue
		}
		status, at, ok := strings.Cut(value, ":")
		unix, err := strconv.ParseInt(at, 10, 64)
		if !ok || err != nil || now.Sub(time.Unix(unix, 0)) > presenceTTL {
			continue
		}
		out[uint(id)] = status
	}
	return out
}

func visitorLiveKey(conversationID uint) string { return fmt.Sprintf("chat:vlive:%d", conversationID) }

// MarkVisitorLive records that a visitor stream for the conversation is
// open, and reports whether it was not before. Streams call it on open and
// on every keep-alive, so a second tab keeps the mark alive after the first
// closes.
func (s *Service) MarkVisitorLive(ctx context.Context, conversationID uint) bool {
	if s.rdb == nil {
		return false
	}
	key := visitorLiveKey(conversationID)
	set, err := s.rdb.SetNX(ctx, key, 1, visitorLiveTTL).Result()
	if err != nil {
		return false
	}
	if !set {
		s.rdb.Expire(ctx, key, visitorLiveTTL)
	}
	return set
}

// ClearVisitorLive records that the visitor's stream closed.
func (s *Service) ClearVisitorLive(ctx context.Context, conversationID uint) {
	if s.rdb != nil {
		s.rdb.Del(ctx, visitorLiveKey(conversationID))
	}
}

// VisitorLive reports whether a stream for the conversation is open on any replica.
func (s *Service) VisitorLive(ctx context.Context, conversationID uint) bool {
	if s.rdb == nil {
		return false
	}
	n, err := s.rdb.Exists(ctx, visitorLiveKey(conversationID)).Result()
	return err == nil && n > 0
}

// allowTyping lets one typing event per side and conversation through every
// few seconds.
func (s *Service) allowTyping(ctx context.Context, conversationID uint, side string) bool {
	if s.rdb == nil {
		return true
	}
	ok, err := s.rdb.SetNX(ctx, fmt.Sprintf("chat:typing:%d:%s", conversationID, side), 1, typingThrottle).Result()
	return err == nil && ok
}
