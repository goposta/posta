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
	"regexp"
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
)

// VisitorAwayAfter is how long a visitor without an open widget must be gone before agent replies go by email.
const VisitorAwayAfter = 2 * time.Minute

// OfflineEmailDelay lets a burst of agent messages become one email.
const OfflineEmailDelay = 60 * time.Second

type OfflineFacts struct {
	VisitorLive     bool
	VisitorLastSeen time.Time
	VisitorEmail    string
	ReplyFrom       string
	Status          models.MessageStatus
	State           models.ChatState
	Now             time.Time
}

// ShouldEmail only follows an agent's message, so nothing a visitor does can send an email.
func ShouldEmail(f OfflineFacts) bool {
	return !f.VisitorLive &&
		f.Now.Sub(f.VisitorLastSeen) >= VisitorAwayAfter &&
		f.VisitorEmail != "" &&
		f.ReplyFrom != "" &&
		f.Status != models.MessageStatusQuarantined &&
		f.State != models.ChatStateSpam
}

var quoteHeader = regexp.MustCompile(`(?im)^(on .{1,200}wrote:|le .{1,200}a écrit\s?:|-{2,}\s*original message\s*-{2,}|from:\s.+)\s*$`)

// StripQuotedReply drops the quote below an "On … wrote:" line, an "Original Message" marker, or trailing "> " lines.
func StripQuotedReply(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	if loc := quoteHeader.FindStringIndex(body); loc != nil {
		body = body[:loc[0]]
	}
	lines := strings.Split(body, "\n")
	end := len(lines)
	for end > 0 {
		line := strings.TrimSpace(lines[end-1])
		if line == "" || strings.HasPrefix(line, ">") {
			end--
			continue
		}
		break
	}
	return strings.TrimSpace(strings.Join(lines[:end], "\n"))
}
