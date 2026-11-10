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
	"slices"
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
)

const (
	ReplyMinutes = "minutes"
	ReplyHours   = "hours"
	ReplyDay     = "day"

	replyWindow     = 30 * 24 * time.Hour
	replyMinSamples = 10
	replyCacheTTL   = 15 * time.Minute
	maxShownAgents  = 3
)

// ReplyBucket rounds a median first response so visitors see a promise, not a statistic.
func ReplyBucket(seconds float64) string {
	switch {
	case seconds < time.Hour.Seconds():
		return ReplyMinutes
	case seconds < 24*time.Hour.Seconds():
		return ReplyHours
	}
	return ReplyDay
}

// ReplyTime is cached per widget so /config, fetched on every page view, never runs the median query itself.
func (s *Service) ReplyTime(ctx context.Context, w *models.ChatWidget) string {
	key := fmt.Sprintf("chat:replytime:%d", w.ID)
	if s.rdb != nil {
		if v, err := s.rdb.Get(ctx, key).Result(); err == nil {
			return strings.TrimPrefix(v, "=")
		}
	}
	bucket := ""
	if median, n, err := s.repo.MedianFirstResponse(w.ID, time.Now().Add(-replyWindow)); err == nil && n >= replyMinSamples {
		bucket = ReplyBucket(median)
	}
	if s.rdb != nil {
		// Prefixed so an empty bucket is still a cache hit.
		_ = s.rdb.Set(ctx, key, "="+bucket, replyCacheTTL).Err()
	}
	return bucket
}

// OnlineAgentNames gives first names only: no email, ID or avatar reaches the customer's page.
func (s *Service) OnlineAgentNames(ctx context.Context, workspaceID uint) []string {
	list, err := s.presence.List(ctx, workspaceID)
	if err != nil {
		return nil
	}
	var ids []uint
	for id, status := range list {
		if status == StatusOnline {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	names, err := s.repo.UserNames(ids)
	if err != nil {
		return nil
	}
	out := make([]string, 0, maxShownAgents)
	for _, n := range names {
		if first := FirstName(n); first != "" {
			out = append(out, first)
		}
		if len(out) == maxShownAgents {
			break
		}
	}
	return out
}

func FirstName(name string) string {
	if f := strings.Fields(CleanLine(name, 80)); len(f) > 0 {
		return f[0]
	}
	return ""
}
