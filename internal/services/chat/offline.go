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

// VisitorAwayAfter is how long a visitor without an open widget must have
// been gone before an agent's reply goes to them by email.
const VisitorAwayAfter = 2 * time.Minute

// OfflineEmailDelay is how long after an agent's last message the follow-up
// email waits, so a burst of messages becomes one email.
const OfflineEmailDelay = 60 * time.Second

// OfflineFacts is what decides whether an agent's reply reaches a visitor by
// email.
type OfflineFacts struct {
	VisitorLive     bool
	VisitorLastSeen time.Time
	VisitorEmail    string
	ReplyFrom       string
	Status          models.MessageStatus
	State           models.ChatState
	Now             time.Time
}

// ShouldEmail is the plan's §7 rule: the visitor has no live widget and has
// been gone a while, gave an email, the widget has a sender, and the
// conversation is not spam. Every such email follows an agent's message;
// nothing a visitor does can send one.
func ShouldEmail(f OfflineFacts) bool {
	return !f.VisitorLive &&
		f.Now.Sub(f.VisitorLastSeen) >= VisitorAwayAfter &&
		f.VisitorEmail != "" &&
		f.ReplyFrom != "" &&
		f.Status != models.MessageStatusQuarantined &&
		f.State != models.ChatStateSpam
}

var quoteHeader = regexp.MustCompile(`(?im)^(on .{1,200}wrote:|le .{1,200}a écrit\s?:|-{2,}\s*original message\s*-{2,}|from:\s.+)\s*$`)

// StripQuotedReply keeps what a visitor wrote in an emailed answer and drops
// the quoted conversation below it: from an "On … wrote:" line, an
// "Original Message" marker, or a trailing block of "> " lines.
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
