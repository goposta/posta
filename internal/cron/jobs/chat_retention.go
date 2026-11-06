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

package jobs

import (
	"context"
	"net/netip"
	"time"

	"github.com/goposta/posta/internal/services/chat"
	"github.com/hibiken/asynq"
	"github.com/jkaninda/logger"
)

// Full IPs are kept this long for abuse handling, then cut to their /24 (IPv4) or /48 (IPv6) network.
const chatIPKeepDays = 30

const chatRetentionBatch = 500

// ChatRetentionJob erases expired closed and spam conversations and orphaned visitors, and truncates old IPs.
type ChatRetentionJob struct {
	svc  *chat.Service
	days int
	now  func() time.Time
}

// NewChatRetentionJob keeps conversations for days (a widget may set fewer); 0 keeps them forever, IPs still truncate.
func NewChatRetentionJob(svc *chat.Service, days int) *ChatRetentionJob {
	return &ChatRetentionJob{svc: svc, days: days, now: time.Now}
}

func (j *ChatRetentionJob) Name() string     { return "chat-retention" }
func (j *ChatRetentionJob) Schedule() string { return "30 3 * * *" }

func (j *ChatRetentionJob) Run(ctx context.Context, _ *asynq.Client) error {
	repo := j.svc.Repo()
	widgets, err := repo.RetentionWidgets()
	if err != nil {
		return err
	}
	var erased int
	for _, w := range widgets {
		days := EffectiveChatRetention(j.days, w.RetentionDays)
		if days <= 0 {
			continue
		}
		cutoff := j.now().AddDate(0, 0, -days)
		for {
			ids, err := repo.ExpiredConversationIDs(w.ID, cutoff, chatRetentionBatch)
			if err != nil {
				return err
			}
			for _, id := range ids {
				keys, err := repo.EraseConversation(id)
				if err != nil {
					logger.Warn("chat-retention: erase failed", "conversation_id", id, "error", err)
					continue
				}
				j.svc.DeleteBlobs(ctx, keys)
				erased++
			}
			if len(ids) < chatRetentionBatch {
				break
			}
		}
	}

	visitors := int64(0)
	if j.days > 0 {
		if visitors, err = repo.DeleteIdleVisitors(j.now().AddDate(0, 0, -j.days)); err != nil {
			logger.Warn("chat-retention: idle visitor cleanup failed", "error", err)
		}
	}
	truncated := j.truncateIPs()
	logger.Info("chat-retention: done", "conversations", erased, "visitors", visitors, "ips_truncated", truncated)
	return nil
}

func (j *ChatRetentionJob) truncateIPs() int {
	repo := j.svc.Repo()
	cutoff := j.now().AddDate(0, 0, -chatIPKeepDays)
	var after uint
	n := 0
	for {
		rows, err := repo.VisitorsWithIPsBefore(cutoff, after, chatRetentionBatch)
		if err != nil {
			logger.Warn("chat-retention: ip truncation failed", "error", err)
			return n
		}
		for _, row := range rows {
			after = row.ID
			first, last := TruncateIP(row.FirstSeenIP), TruncateIP(row.LastSeenIP)
			if first == row.FirstSeenIP && last == row.LastSeenIP {
				continue
			}
			if err := repo.UpdateVisitor(row.ID, map[string]any{"first_seen_ip": first, "last_seen_ip": last}); err == nil {
				n++
			}
		}
		if len(rows) < chatRetentionBatch {
			return n
		}
	}
}

// EffectiveChatRetention is the platform's retention, lowered by a widget's.
func EffectiveChatRetention(platformDays, widgetDays int) int {
	if widgetDays > 0 && (platformDays <= 0 || widgetDays < platformDays) {
		return widgetDays
	}
	return platformDays
}

// TruncateIP keeps the /24 of an IPv4 address or the /48 of an IPv6 one, and drops anything unparseable.
func TruncateIP(ip string) string {
	if ip == "" {
		return ""
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return ""
	}
	addr = addr.Unmap()
	bits := 48
	if addr.Is4() {
		bits = 24
	}
	prefix, err := addr.Prefix(bits)
	if err != nil {
		return ""
	}
	return prefix.Addr().String()
}
