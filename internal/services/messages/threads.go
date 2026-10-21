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

package messages

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/realtime"
	"github.com/jkaninda/logger"
	"gorm.io/gorm"
)

// threadLocalPrefix is the local-part prefix of a thread address,
// msg+<token>@<inbound domain>.
const threadLocalPrefix = "msg+"

// threadTokenLength is the length of newThreadToken's output: 16 random bytes
// in unpadded base32.
const threadTokenLength = 26

// ThreadStore is the slice of MessageRepository the router needs.
type ThreadStore interface {
	FindByThreadToken(token string) (*models.Message, error)
	AddInboundReply(reply *models.MessageReply) (bool, error)
	UpdateFields(id uint, fields map[string]any) error
}

// ReplyEnqueuer schedules the follow-up work (webhook, notification) for a
// reply attached to a thread.
type ReplyEnqueuer interface {
	EnqueueMessageReply(replyID uint) error
}

// ThreadRouter attaches inbound email sent to a thread address onto the form
// message it belongs to. It implements inbound.ThreadRouter.
type ThreadRouter struct {
	store    ThreadStore
	domain   string
	realtime realtime.Bus
	enqueuer ReplyEnqueuer
}

// NewThreadRouter returns a router for thread addresses on inboundDomain.
func NewThreadRouter(store ThreadStore, inboundDomain string) *ThreadRouter {
	return &ThreadRouter{store: store, domain: strings.ToLower(strings.TrimSpace(inboundDomain))}
}

func (r *ThreadRouter) SetRealtime(b realtime.Bus)  { r.realtime = b }
func (r *ThreadRouter) SetEnqueuer(e ReplyEnqueuer) { r.enqueuer = e }

// ParseThreadAddress returns the thread token in addr when it is a thread
// address on domain. It accepts a bare address or a "Name <addr>" form.
func ParseThreadAddress(addr, domain string) (string, bool) {
	addr = strings.TrimSpace(addr)
	if parsed, err := mail.ParseAddress(addr); err == nil {
		addr = parsed.Address
	}
	addr = strings.ToLower(addr)
	at := strings.LastIndex(addr, "@")
	if at < 0 || domain == "" || addr[at+1:] != strings.ToLower(domain) {
		return "", false
	}
	local := addr[:at]
	if !strings.HasPrefix(local, threadLocalPrefix) {
		return "", false
	}
	token := local[len(threadLocalPrefix):]
	if len(token) != threadTokenLength {
		return "", false
	}
	for _, c := range token {
		if (c < 'a' || c > 'z') && (c < '2' || c > '7') {
			return "", false
		}
	}
	return token, true
}

// RouteReply attaches rec to a thread when one of recipients is a thread
// address. It reports false, with no side effects, when no recipient names a
// thread of rec's own workspace; the caller then handles rec as ordinary
// inbound mail.
func (r *ThreadRouter) RouteReply(ctx context.Context, rec *models.InboundEmail, recipients []string) (bool, error) {
	if r == nil || r.domain == "" || rec == nil {
		return false, nil
	}
	for _, rcpt := range recipients {
		token, ok := ParseThreadAddress(rcpt, r.domain)
		if !ok {
			continue
		}
		msg, err := r.store.FindByThreadToken(token)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return false, err
		}
		// A thread token only routes within its own workspace: mail that reached
		// workspace A's domain must never write into a workspace B thread.
		if !sameWorkspace(rec.WorkspaceID, msg.WorkspaceID) {
			logger.Warn("thread reply rejected: workspace mismatch",
				"inbound_id", rec.UUID, "message_id", msg.UUID)
			continue
		}
		if err := r.attach(ctx, rec, msg, rcpt); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func (r *ThreadRouter) attach(ctx context.Context, rec *models.InboundEmail, msg *models.Message, rcpt string) error {
	inboundID := rec.ID
	reply := &models.MessageReply{
		MessageID:      msg.ID,
		WorkspaceID:    msg.WorkspaceID,
		Kind:           models.MessageReplyKindInbound,
		FromAddr:       rec.Sender,
		ToAddr:         rcpt,
		Subject:        rec.Subject,
		TextBody:       rec.TextBody,
		HTMLBody:       rec.HTMLBody,
		InboundEmailID: &inboundID,
		SenderMismatch: !sameAddress(rec.Sender, msg.SenderEmail),
	}
	if rec.MessageID != "" {
		reply.MessageIDHeader = "<" + strings.Trim(rec.MessageID, "<>") + ">"
	}
	created, err := r.store.AddInboundReply(reply)
	if err != nil {
		return err
	}
	if !created {
		// A retried parse task: this email is already on the thread.
		return nil
	}

	now := time.Now()
	fields := map[string]any{"read_at": nil, "updated_at": now}
	if msg.State != models.MessageStateSpam {
		fields["state"] = models.MessageStateOpen
		msg.State = models.MessageStateOpen
	}
	if err := r.store.UpdateFields(msg.ID, fields); err != nil {
		logger.Error("failed to reopen thread after reply", "message_id", msg.ID, "error", err)
	}

	PublishEvent(ctx, r.realtime, EventReplyReceived, msg)
	if r.enqueuer != nil {
		if err := r.enqueuer.EnqueueMessageReply(reply.ID); err != nil {
			logger.Error("failed to enqueue message reply processing", "reply_id", reply.ID, "error", err)
		}
	}
	return nil
}

func sameWorkspace(a, b *uint) bool {
	return a != nil && b != nil && *a == *b
}

func sameAddress(a, b string) bool {
	na := NormalizeEmail(a)
	return na != "" && na == NormalizeEmail(b)
}
