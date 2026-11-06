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
	"errors"
	"fmt"
	"html"
	"net/mail"
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/email"
	"github.com/goposta/posta/internal/services/messages"
	"github.com/jkaninda/logger"
	"gorm.io/gorm"
)

// chatThreadPrefix is the local part of a conversation's reply address,
// chat+<token>@<inbound domain>; forms use msg+.
const chatThreadPrefix = "chat+"

// SendOfflineEmail emails the agent messages a visitor who left has not seen,
// as one email, from the widget's reply sender and as the agent who wrote
// last. It runs as a delayed task for the agent message with sequence number
// seq, and does nothing when a later agent message will run its own, when the
// visitor came back, or when nothing is left to send.
func (s *Service) SendOfflineEmail(ctx context.Context, conversationID uint, seq int64) error {
	if s.mailer == nil || s.users == nil {
		return nil
	}
	conv, err := s.repo.FindConversationByID(conversationID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	latest, err := s.repo.LatestSeqBy(conv.ID, models.ChatAuthorAgent)
	if err != nil {
		return err
	}
	if latest > seq || conv.Visitor == nil || conv.WorkspaceID == nil {
		return nil
	}
	w, err := s.repo.FindWidgetByID(conv.WidgetID)
	if err != nil {
		return nil
	}
	if !ShouldEmail(OfflineFacts{
		VisitorLive:     s.VisitorLive(ctx, conv.ID),
		VisitorLastSeen: conv.Visitor.LastSeenAt,
		VisitorEmail:    conv.Visitor.Email,
		ReplyFrom:       w.ReplyFrom,
		Status:          conv.Status,
		State:           conv.State,
		Now:             time.Now(),
	}) {
		return nil
	}
	// The visitor's read receipt says what they saw in the widget; a message
	// that arrived while the panel was closed is still news.
	items, err := s.repo.AgentMessagesToEmail(conv.ID, max(conv.EmailedThroughSeq, conv.VisitorReadSeq))
	if err != nil || len(items) == 0 {
		return err
	}
	last := items[len(items)-1]
	if last.AuthorID == nil {
		return nil
	}
	agent, err := s.users.FindByID(*last.AuthorID)
	if err != nil {
		return nil
	}

	req := s.offlineEmail(conv, w, items)
	resp, err := s.mailer.Send(ctx, agent.ID, 0, conv.WorkspaceID, agent.Email, req)
	if err != nil {
		return fmt.Errorf("send chat follow-up for conversation %d: %w", conv.ID, err)
	}
	ids := make([]uint, 0, len(items))
	for _, m := range items {
		ids = append(ids, m.ID)
	}
	if err := s.repo.MarkEmailed(conv.ID, ids, last.Seq, resp.ID); err != nil {
		logger.Error("chat: failed to record follow-up email", "conversation_id", conv.ID, "error", err)
	}
	return nil
}

// offlineEmail builds the follow-up email. Bodies are plain text, escaped
// for the HTML part.
func (s *Service) offlineEmail(conv *models.ChatConversation, w *models.ChatWidget, items []models.ChatMessage) *email.SendRequest {
	from := w.ReplyFrom
	if w.ReplyFromName != "" {
		from = (&mail.Address{Name: messages.SanitizeHeaderValue(w.ReplyFromName), Address: w.ReplyFrom}).String()
	}
	title := w.Title
	if title == "" {
		title = w.DisplayName
	}
	replyTo := ""
	if s.cfg.InboundDomain != "" && conv.ThreadToken != nil {
		replyTo = chatThreadPrefix + *conv.ThreadToken + "@" + s.cfg.InboundDomain
	}
	link := continueURL(conv)

	var text, body strings.Builder
	for _, m := range items {
		name := "Support"
		if w.ShowAgentNames && m.Author != nil && m.Author.Name != "" {
			name = m.Author.Name
		}
		fmt.Fprintf(&text, "%s:\n%s\n\n", name, m.Body)
		fmt.Fprintf(&body, `<p style="margin:0 0 4px;color:#6b7280;font-size:12px">%s</p><div style="margin:0 0 16px;padding:12px 14px;border-radius:10px;background:#f3f4f6;white-space:pre-wrap">%s</div>`,
			html.EscapeString(name), html.EscapeString(m.Body))
	}
	switch {
	case replyTo != "":
		text.WriteString("Reply to this email to answer.")
		body.WriteString(`<p style="color:#6b7280;font-size:13px">Reply to this email to answer.`)
	case link != "":
		fmt.Fprintf(&text, "Continue the conversation in the chat: %s", link)
		fmt.Fprintf(&body, `<p style="color:#6b7280;font-size:13px">Continue the conversation in the chat: <a href="%s">%s</a>`,
			html.EscapeString(link), html.EscapeString(link))
	}
	if replyTo != "" && link != "" {
		fmt.Fprintf(&text, " Or continue in the chat: %s", link)
		fmt.Fprintf(&body, ` Or <a href="%s">continue in the chat</a>.`, html.EscapeString(link))
	}
	body.WriteString("</p>")

	req := &email.SendRequest{
		From:    from,
		To:      []string{conv.Visitor.Email},
		Subject: messages.SanitizeHeaderValue("New reply from " + title),
		Text:    text.String(),
		HTML:    `<div style="font-family:system-ui,sans-serif;font-size:15px;line-height:1.5;color:#111827">` + body.String() + `</div>`,
	}
	if replyTo != "" {
		req.SystemHeaders = map[string]string{"Reply-To": replyTo}
	}
	return req
}

// continueURL is the page the visitor chatted on, with a fragment that opens
// the chat. The same browser resumes the conversation from its stored token.
func continueURL(conv *models.ChatConversation) string {
	page := conv.StartPageURL
	if conv.Visitor != nil && conv.Visitor.LastPageURL != "" {
		page = conv.Visitor.LastPageURL
	}
	if page == "" {
		return ""
	}
	if i := strings.IndexByte(page, '#'); i >= 0 {
		page = page[:i]
	}
	return page + "#posta-chat=" + conv.UUID
}

// ParseChatThreadAddress returns the token in a conversation's reply
// address on domain.
func ParseChatThreadAddress(addr, domain string) (string, bool) {
	if parsed, err := mail.ParseAddress(strings.TrimSpace(addr)); err == nil {
		addr = parsed.Address
	}
	addr = strings.ToLower(strings.TrimSpace(addr))
	at := strings.LastIndex(addr, "@")
	if at < 0 || domain == "" || addr[at+1:] != strings.ToLower(domain) {
		return "", false
	}
	local := addr[:at]
	if !strings.HasPrefix(local, chatThreadPrefix) {
		return "", false
	}
	token := local[len(chatThreadPrefix):]
	if len(token) != 26 {
		return "", false
	}
	for _, c := range token {
		if (c < 'a' || c > 'z') && (c < '2' || c > '7') {
			return "", false
		}
	}
	return token, true
}

// RouteReply adds a visitor's emailed answer to their conversation when one
// of the recipients is its reply address. It implements inbound.ThreadRouter
// and reports false when no recipient is a conversation of rec's workspace.
func (s *Service) RouteReply(ctx context.Context, rec *models.InboundEmail, recipients []string) (bool, error) {
	if s == nil || s.cfg.InboundDomain == "" || rec == nil {
		return false, nil
	}
	for _, rcpt := range recipients {
		token, ok := ParseChatThreadAddress(rcpt, s.cfg.InboundDomain)
		if !ok {
			continue
		}
		conv, err := s.repo.FindConversationByThreadToken(token)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return false, err
		}
		// A token only routes within its workspace: mail that reached
		// workspace A's domain never writes into workspace B's chat.
		if rec.WorkspaceID == nil || conv.WorkspaceID == nil || *rec.WorkspaceID != *conv.WorkspaceID {
			logger.Warn("chat: emailed reply rejected: workspace mismatch", "inbound_id", rec.UUID)
			continue
		}
		return true, s.attachEmailReply(ctx, conv, rec)
	}
	return false, nil
}

func (s *Service) attachEmailReply(ctx context.Context, conv *models.ChatConversation, rec *models.InboundEmail) error {
	body := StripQuotedReply(rec.TextBody)
	if body == "" {
		// Nothing of the visitor's own above the quote: routed, not stored.
		return nil
	}
	w, err := s.repo.FindWidgetByID(conv.WidgetID)
	if err != nil {
		return nil
	}
	if limit := w.MessageLimit(); len(body) > limit {
		body = strings.ToValidUTF8(body[:limit], "")
	}
	msg := &models.ChatMessage{
		WorkspaceID:    conv.WorkspaceID,
		ConversationID: conv.ID,
		AuthorKind:     models.ChatAuthorVisitor,
		Body:           CleanBody(body),
		Channel:        "email",
		// One stored message per inbound email, however often its parse is retried.
		ClientMsgID: fmt.Sprintf("inbound-%d", rec.ID),
	}
	duplicate, err := s.repo.AppendMessage(msg)
	if err != nil || duplicate {
		return err
	}
	if conv.State == models.ChatStatePending || conv.State == models.ChatStateClosed {
		conv.State, conv.ClosedAt = models.ChatStateOpen, nil
	}
	conv.LastSeq = msg.Seq
	if conv.Status != models.MessageStatusQuarantined && conv.State != models.ChatStateSpam {
		s.publishConversation(ctx, conv, false, "")
		s.publishMessage(ctx, conv, w, msg)
		s.scheduleUnanswered(conv, msg.Seq)
		s.webhook(WebhookMessage, conv, msg.ID)
	}
	return nil
}
