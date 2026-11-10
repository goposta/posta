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

// Package chat is live chat: Postgres holds every sequenced message; Redis pub/sub only tells streams something is new.
package chat

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"sync/atomic"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/email"
	"github.com/goposta/posta/internal/services/messages"
	"github.com/goposta/posta/internal/services/messagescan"
	"github.com/goposta/posta/internal/services/ratelimit"
	"github.com/goposta/posta/internal/services/realtime"
	"github.com/goposta/posta/internal/storage/blob"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/logger"
	"github.com/redis/go-redis/v9"
)

var (
	ErrEmptyBody     = errors.New("message is empty")
	ErrBodyTooLong   = errors.New("message is too long")
	ErrNameRequired  = errors.New("name is required")
	ErrEmailRequired = errors.New("email is required")
	ErrInvalidEmail  = errors.New("email is not a valid address")
	ErrRateLimited   = errors.New("too many messages")
)

// Long enough to stop a script that posts on page load, short enough that no person waits for it.
const MinFillSeconds = 2

// A non-positive limit disables that quota.
type Config struct {
	StartsPerIPHourly       int
	StartsPerWorkspaceDaily int
	VisitorPerMinute        int
	VisitorDaily            int
	ConversationPerMinute   int
	MaxStreams              int
	// UnansweredAfter is how long a visitor's message waits before the assignee or admins are told; 0 disables it.
	UnansweredAfter time.Duration
	// Visitors' emailed answers go to chat+<token>@InboundDomain; empty means they reply in the chat only.
	InboundDomain string
}

func DefaultConfig() Config {
	return Config{
		StartsPerIPHourly:       5,
		StartsPerWorkspaceDaily: 2000,
		VisitorPerMinute:        30,
		VisitorDaily:            500,
		ConversationPerMinute:   60,
		MaxStreams:              5000,
		UnansweredAfter:         5 * time.Minute,
	}
}

type Enqueuer interface {
	EnqueueChatNotify(conversationID uint) error
	EnqueueChatOfflineEmail(conversationID uint, seq int64, delay time.Duration) error
	EnqueueChatUnanswered(conversationID uint, seq int64, delay time.Duration) error
	EnqueueChatWebhook(event string, conversationID, messageID uint) error
}

// Limits are a workspace's plan limits for chat; 0 is unlimited.
type Limits struct {
	MaxAgents            int
	MonthlyConversations int
}

type LimitsProvider func(workspaceID uint) Limits

var ErrAgentLimit = errors.New("the plan's number of chat agents are already online")

type Mailer interface {
	Send(ctx context.Context, userID, apiKeyID uint, workspaceID *uint, userEmail string, req *email.SendRequest) (*email.SendResponse, error)
}

type quotaChecker interface {
	Check(ctx context.Context, quotas []ratelimit.Quota) error
}

type Service struct {
	repo     *repositories.ChatRepository
	rdb      *redis.Client
	presence *Presence
	mailer   Mailer
	blobs    blob.Store
	limits   LimitsProvider
	// licensed gates Enterprise parts such as auto-assign; nil means Community.
	licensed   func() bool
	workspaces *repositories.WorkspaceRepository
	users      *repositories.UserRepository
	bus        realtime.Bus
	quotas     quotaChecker
	scanner    *messagescan.Scanner
	nonces     *messages.Nonces
	enqueuer   Enqueuer
	cfg        Config

	streams atomic.Int64
}

func NewService(
	repo *repositories.ChatRepository,
	filterRepo *repositories.MessageFilterRepository,
	suppressionRepo *repositories.SuppressionRepository,
	rdb *redis.Client,
	hmacKey []byte,
	cfg Config,
) *Service {
	s := &Service{
		repo:     repo,
		rdb:      rdb,
		presence: NewPresence(rdb),
		cfg:      cfg,
		nonces:   messages.NewNonces(hmacKey, rdb, "chat:nonce:"),
		scanner: messagescan.New(&lookup{
			repo: repo, filterRepo: filterRepo, suppressionRepo: suppressionRepo,
		}),
	}
	if rdb != nil {
		s.quotas = ratelimit.NewQuotas(rdb)
		s.bus = realtime.NewRedis(rdb)
	}
	return s
}

func (s *Service) SetRealtime(b realtime.Bus) { s.bus = b }
func (s *Service) SetEnqueuer(e Enqueuer)     { s.enqueuer = e }

func (s *Service) SetMailer(m Mailer, users *repositories.UserRepository) {
	s.mailer, s.users = m, users
}

func (s *Service) Presence() *Presence { return s.presence }

// A license past its grace period still returns true: the feature keeps running, only its settings freeze.
func (s *Service) SetLicensed(fn func() bool) { s.licensed = fn }

func (s *Service) SetLimits(l LimitsProvider) { s.limits = l }

func (s *Service) limitsFor(workspaceID uint) Limits {
	if s.limits == nil {
		return Limits{}
	}
	return s.limits(workspaceID)
}

func (s *Service) Repo() *repositories.ChatRepository { return s.repo }
func (s *Service) Bus() realtime.Bus                  { return s.bus }
func (s *Service) Nonces() *messages.Nonces           { return s.nonces }

func (s *Service) AcquireStream() bool {
	if s.cfg.MaxStreams <= 0 {
		s.streams.Add(1)
		return true
	}
	if s.streams.Add(1) > int64(s.cfg.MaxStreams) {
		s.streams.Add(-1)
		return false
	}
	return true
}

func (s *Service) ReleaseStream() { s.streams.Add(-1) }

type StartRequest struct {
	Widget *models.ChatWidget
	// Visitor is nil for a new visitor.
	Visitor     *models.ChatVisitor
	Name        string
	Email       string
	Body        string
	ClientMsgID string
	Nonce       string
	PageURL     string
	ClientIP    string
	UserAgent   string
	// Identity is a signed identity from PostaChat.identify(), if any.
	Identity *Identity
}

type StartResult struct {
	Visitor *models.ChatVisitor
	// Token is set only when this request created the visitor.
	Token        string
	Conversation *models.ChatConversation
	Message      *models.ChatMessage
	// Rejected means the scan refused the message: nothing was stored, and the caller answers as if it had been.
	Rejected bool
}

func (s *Service) Start(ctx context.Context, req StartRequest) (*StartResult, error) {
	w := req.Widget
	body, err := checkBody(req.Body, w.MessageLimit())
	if err != nil {
		return nil, err
	}
	name := CleanLine(req.Name, 120)
	email := strings.ToLower(CleanLine(req.Email, 254))
	if email != "" {
		if addr, err := mail.ParseAddress(email); err != nil || addr.Address != email {
			return nil, ErrInvalidEmail
		}
	}
	if w.RequireName && name == "" && (req.Visitor == nil || req.Visitor.Name == "") {
		return nil, ErrNameRequired
	}
	needEmail := w.RequireEmail || !s.Online(ctx, w)
	if needEmail && email == "" && (req.Visitor == nil || req.Visitor.Email == "") {
		return nil, ErrEmailRequired
	}

	// A banned visitor or IP is told its message arrived, before anything else can answer differently.
	if s.ipBanned(ctx, *w.WorkspaceID, req.ClientIP) || (req.Visitor != nil && req.Visitor.BannedAt != nil) {
		return &StartResult{Rejected: true}, nil
	}

	quotas := []ratelimit.Quota{
		{Key: fmt.Sprintf("chat:ip:start:%d:%s", w.ID, req.ClientIP), Limit: s.cfg.StartsPerIPHourly, Window: time.Hour},
		{Key: fmt.Sprintf("chat:ws:start:%d", *w.WorkspaceID), Limit: s.cfg.StartsPerWorkspaceDaily, Window: 24 * time.Hour},
	}
	if req.ClientIP == "" {
		quotas[0].Key = ""
	}
	if req.Visitor != nil {
		quotas = append(quotas, s.visitorQuotas(req.Visitor.ID)...)
	}
	// The public path fails closed: an unreadable counter is a refusal.
	if err := s.check(ctx, quotas); err != nil {
		return nil, err
	}

	nonceFailed := s.nonces.Verify(ctx, w.PublicKey, req.Nonce, MinFillSeconds) != nil
	verdict := s.scanner.Scan(messagescan.Input{
		Policy:      w,
		WorkspaceID: *w.WorkspaceID,
		SenderEmail: email,
		SenderName:  name,
		Body:        body,
		ClientIP:    req.ClientIP,
		UserAgent:   req.UserAgent,
		NonceFailed: nonceFailed,
	})
	if verdict.Action == messagescan.ActionReject {
		logger.Info("chat: first message rejected", "widget_id", w.ID, "score", verdict.Score, "reasons", verdict.Reasons)
		return &StartResult{Rejected: true}, nil
	}

	res := &StartResult{Visitor: req.Visitor}
	page := CleanPageURL(req.PageURL)
	if res.Visitor == nil {
		token, hash, err := NewVisitorToken()
		if err != nil {
			return nil, err
		}
		res.Token = token
		res.Visitor = &models.ChatVisitor{
			WorkspaceID: w.WorkspaceID,
			WidgetID:    w.ID,
			TokenHash:   hash,
			Name:        name,
			Email:       email,
			FirstSeenIP: req.ClientIP,
			LastSeenIP:  req.ClientIP,
			UserAgent:   truncate(req.UserAgent, 500),
			LastPageURL: page,
			LastSeenAt:  time.Now(),
		}
		if err := s.repo.CreateVisitor(res.Visitor); err != nil {
			return nil, err
		}
	} else {
		if err := s.repo.UpdateVisitorContact(res.Visitor.ID, name, email); err != nil {
			logger.Warn("chat: failed to update visitor", "visitor_id", res.Visitor.ID, "error", err)
		}
		if name != "" {
			res.Visitor.Name = name
		}
		if email != "" {
			res.Visitor.Email = email
		}
		s.touch(res.Visitor, req.ClientIP, page)
	}

	if req.Identity != nil {
		// An identity that does not verify leaves the visitor anonymous.
		s.Identify(w, res.Visitor, req.Identity)
	}

	conv := &models.ChatConversation{
		WorkspaceID:  w.WorkspaceID,
		WidgetID:     w.ID,
		VisitorID:    res.Visitor.ID,
		State:        models.ChatStateOpen,
		Status:       verdict.Status(),
		SpamScore:    verdict.Score,
		ScanReasons:  verdict.Reasons,
		StartPageURL: page,
		ThreadToken:  newThreadToken(),
	}
	msg := &models.ChatMessage{
		AuthorKind:  models.ChatAuthorVisitor,
		Body:        body,
		ClientMsgID: cleanClientID(req.ClientMsgID),
	}
	if err := s.repo.CreateConversation(conv, msg); err != nil {
		return nil, err
	}
	conv.Visitor = res.Visitor
	res.Conversation, res.Message = conv, msg

	if conv.Status != models.MessageStatusQuarantined {
		s.autoAssign(ctx, w, conv)
		s.publishConversation(ctx, conv, true, preview(body))
		s.publishMessage(ctx, conv, w, msg)
		if s.enqueuer != nil {
			if err := s.enqueuer.EnqueueChatNotify(conv.ID); err != nil {
				logger.Error("chat: failed to enqueue notification", "conversation_id", conv.ID, "error", err)
			}
		}
		s.scheduleUnanswered(conv, msg.Seq)
		s.webhook(WebhookStarted, conv, msg.ID)
		s.webhook(WebhookMessage, conv, msg.ID)
		if conv.AssignedToID != nil {
			s.webhook(WebhookAssigned, conv, 0)
		}
	}
	return res, nil
}

func (s *Service) PostVisitorMessage(ctx context.Context, w *models.ChatWidget, v *models.ChatVisitor, conv *models.ChatConversation, body, clientMsgID, ip, page string) (*models.ChatMessage, error) {
	body, err := checkBody(body, w.MessageLimit())
	if err != nil {
		return nil, err
	}
	// A banned visitor, or a line the scan rejects, is answered as if stored.
	if v.BannedAt != nil {
		return discarded(body, clientMsgID), nil
	}
	quotas := append(s.visitorQuotas(v.ID), s.conversationQuota(conv.ID))
	if err := s.check(ctx, quotas); err != nil {
		return nil, err
	}
	if s.scanLater(ctx, w, conv, body) {
		return discarded(body, clientMsgID), nil
	}
	return s.appendVisitor(ctx, w, v, conv, &models.ChatMessage{
		WorkspaceID:    conv.WorkspaceID,
		ConversationID: conv.ID,
		AuthorKind:     models.ChatAuthorVisitor,
		Body:           body,
		ClientMsgID:    cleanClientID(clientMsgID),
	}, ip, page)
}

func (s *Service) appendVisitor(ctx context.Context, w *models.ChatWidget, v *models.ChatVisitor, conv *models.ChatConversation, msg *models.ChatMessage, ip, page string) (*models.ChatMessage, error) {
	duplicate, err := s.repo.AppendMessage(msg)
	if err != nil {
		return nil, err
	}
	s.touch(v, ip, CleanPageURL(page))
	if !duplicate && conv.Status != models.MessageStatusQuarantined && conv.State != models.ChatStateSpam {
		if conv.State == models.ChatStatePending || conv.State == models.ChatStateClosed {
			// The visitor's message reopened it.
			conv.State, conv.ClosedAt, conv.LastSeq = models.ChatStateOpen, nil, msg.Seq
			s.publishConversation(ctx, conv, false, "")
		}
		s.publishMessage(ctx, conv, w, msg)
		s.scheduleUnanswered(conv, msg.Seq)
		s.webhook(WebhookMessage, conv, msg.ID)
	}
	return msg, nil
}

type Agent struct {
	ID   uint
	Name string
}

// Replying to an unassigned conversation claims it for the agent.
func (s *Service) PostAgentMessage(ctx context.Context, conv *models.ChatConversation, agent Agent, body, clientMsgID string) (*models.ChatMessage, error) {
	w, err := s.repo.FindWidgetByID(conv.WidgetID)
	if err != nil {
		return nil, err
	}
	body, err = checkBody(body, w.MessageLimit())
	if err != nil {
		return nil, err
	}
	return s.appendAgent(ctx, w, conv, agent, &models.ChatMessage{
		WorkspaceID:    conv.WorkspaceID,
		ConversationID: conv.ID,
		AuthorKind:     models.ChatAuthorAgent,
		AuthorID:       &agent.ID,
		Body:           body,
		ClientMsgID:    cleanClientID(clientMsgID),
	})
}

func (s *Service) appendAgent(ctx context.Context, w *models.ChatWidget, conv *models.ChatConversation, agent Agent, msg *models.ChatMessage) (*models.ChatMessage, error) {
	// The conversation counter catches loops between automated sides, but an unreadable one never stops a person replying.
	if s.quotas != nil {
		if err := s.quotas.Check(ctx, []ratelimit.Quota{s.conversationQuota(conv.ID)}); errors.Is(err, ratelimit.ErrQuotaExceeded) {
			return nil, ErrRateLimited
		}
	}
	duplicate, err := s.repo.AppendMessage(msg)
	if err != nil {
		return nil, err
	}
	if duplicate {
		return msg, nil
	}
	msg.Author = &models.ActorRef{ID: agent.ID, Name: agent.Name}
	conv.LastSeq = msg.Seq
	if conv.AssignedToID == nil {
		if err := s.repo.UpdateConversationFields(conv.ID, map[string]any{"assigned_to_id": agent.ID}); err == nil {
			conv.AssignedToID = &agent.ID
			conv.AssignedTo = &models.ActorRef{ID: agent.ID, Name: agent.Name}
			s.publishConversation(ctx, conv, false, "")
			s.webhook(WebhookAssigned, conv, 0)
		}
	}
	s.publishMessage(ctx, conv, w, msg)
	s.scheduleOfflineEmail(conv, w, msg.Seq)
	s.webhook(WebhookMessage, conv, msg.ID)
	return msg, nil
}

func (s *Service) SetState(ctx context.Context, conv *models.ChatConversation, state models.ChatState) error {
	fields := map[string]any{"state": state}
	var closedAt *time.Time
	if state == models.ChatStateClosed || state == models.ChatStateSpam {
		now := time.Now()
		closedAt = &now
	}
	fields["closed_at"] = closedAt
	if err := s.repo.UpdateConversationFields(conv.ID, fields); err != nil {
		return err
	}
	wasClosed := conv.State == models.ChatStateClosed
	conv.State, conv.ClosedAt = state, closedAt
	s.publishConversation(ctx, conv, false, "")
	if state == models.ChatStateClosed && !wasClosed {
		s.webhook(WebhookClosed, conv, 0)
	}
	return nil
}

// Assign gives a conversation to a member, or to no one when userID is nil.
func (s *Service) Assign(ctx context.Context, conv *models.ChatConversation, userID *uint) error {
	if err := s.repo.UpdateConversationFields(conv.ID, map[string]any{"assigned_to_id": userID}); err != nil {
		return err
	}
	conv.AssignedToID = userID
	s.publishConversation(ctx, conv, false, "")
	s.webhook(WebhookAssigned, conv, 0)
	return nil
}

// A workspace over its monthly conversations reads offline too, so new visitors leave a message rather than lose one.
func (s *Service) Online(ctx context.Context, w *models.ChatWidget) bool {
	if w.WorkspaceID == nil || !s.inHours(w, time.Now()) {
		return false
	}
	if limit := s.limitsFor(*w.WorkspaceID).MonthlyConversations; limit > 0 {
		now := time.Now().UTC()
		monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		if n, err := s.repo.CountConversationsSince(*w.WorkspaceID, monthStart); err == nil && n >= int64(limit) {
			return false
		}
	}
	return s.presence.AnyOnline(ctx, *w.WorkspaceID)
}

// Business hours are Enterprise; without a license only agent presence decides.
func (s *Service) inHours(w *models.ChatWidget, now time.Time) bool {
	return !s.Licensed() || w.BusinessHours.Open(now)
}

// Licensed reports whether Enterprise chat features run; a lapsed license keeps them running.
func (s *Service) Licensed() bool { return s.licensed != nil && s.licensed() }

func (s *Service) SetPresence(ctx context.Context, workspaceID, userID uint, status string) error {
	if max := s.limitsFor(workspaceID).MaxAgents; max > 0 && status != StatusOffline {
		if list, err := s.presence.List(ctx, workspaceID); err == nil {
			if _, already := list[userID]; !already && len(list) >= max {
				return ErrAgentLimit
			}
		}
	}
	changed, err := s.presence.Set(ctx, workspaceID, userID, status)
	if err != nil || !changed || s.bus == nil {
		return err
	}
	s.publish(ctx, realtime.Event{
		Type: EventPresence, WorkspaceID: workspaceID, Topic: realtime.WorkspaceTopic(workspaceID),
		Data: PresenceEvent{UserID: userID, Status: status},
	})
	return nil
}

// Typing is throttled and never stored.
func (s *Service) Typing(ctx context.Context, conv *models.ChatConversation, visitor bool, name string) {
	side := models.ChatAuthorAgent
	if visitor {
		side = models.ChatAuthorVisitor
	}
	if s.bus == nil || conv.WorkspaceID == nil || !s.allowTyping(ctx, conv.ID, side) {
		return
	}
	ws := *conv.WorkspaceID
	if visitor {
		s.publish(ctx, realtime.Event{Type: EventTyping, WorkspaceID: ws, Topic: realtime.WorkspaceTopic(ws),
			Data: TypingEvent{ConversationID: conv.UUID, Author: side}})
		return
	}
	s.publish(ctx, realtime.Event{Type: EventTyping, WorkspaceID: ws, Topic: realtime.ChatConversationTopic(conv.ID),
		Data: TypingEvent{ConversationID: conv.UUID, Author: side, Name: name}})
}

func (s *Service) MarkRead(ctx context.Context, conv *models.ChatConversation, visitor bool, seq int64) (int64, error) {
	stored, err := s.repo.MarkRead(conv.ID, visitor, seq)
	if err != nil || s.bus == nil || conv.WorkspaceID == nil {
		return stored, err
	}
	ws := *conv.WorkspaceID
	reader, topic := models.ChatAuthorAgent, realtime.ChatConversationTopic(conv.ID)
	if visitor {
		reader, topic = models.ChatAuthorVisitor, realtime.WorkspaceTopic(ws)
	}
	s.publish(ctx, realtime.Event{Type: EventRead, WorkspaceID: ws, Topic: topic,
		Data: ReadEvent{ConversationID: conv.UUID, Reader: reader, Seq: stored}})
	return stored, nil
}

// Visitor presence decides whether agent replies go out by email.
func (s *Service) AnnounceVisitor(ctx context.Context, conv *models.ChatConversation, live bool) {
	if s.bus == nil || conv.WorkspaceID == nil {
		return
	}
	ws := *conv.WorkspaceID
	s.publish(ctx, realtime.Event{Type: EventVisitor, WorkspaceID: ws, Topic: realtime.WorkspaceTopic(ws),
		Data: VisitorEvent{ConversationID: conv.UUID, Live: live}})
}

// Describe fills computed fields: whether the visitor is connected and whether a reply now goes by email.
func (s *Service) Describe(ctx context.Context, conv *models.ChatConversation) {
	conv.VisitorLive = s.VisitorLive(ctx, conv.ID)
	if conv.Visitor == nil {
		return
	}
	w, err := s.repo.FindWidgetByID(conv.WidgetID)
	if err != nil {
		return
	}
	conv.ReplyByEmail = s.Licensed() && ShouldEmail(OfflineFacts{
		VisitorLive:     conv.VisitorLive,
		VisitorLastSeen: conv.Visitor.LastSeenAt,
		VisitorEmail:    conv.Visitor.Email,
		ReplyFrom:       w.ReplyFrom,
		Status:          conv.Status,
		State:           conv.State,
		Now:             time.Now(),
	})
}

// Sending is decided when the task runs: the visitor may be back, or a later agent message's task sends one email for both.
func (s *Service) scheduleOfflineEmail(conv *models.ChatConversation, w *models.ChatWidget, seq int64) {
	if s.enqueuer == nil || !s.Licensed() || w.ReplyFrom == "" || conv.Status == models.MessageStatusQuarantined || conv.State == models.ChatStateSpam {
		return
	}
	if err := s.enqueuer.EnqueueChatOfflineEmail(conv.ID, seq, OfflineEmailDelay); err != nil {
		logger.Error("chat: failed to enqueue offline email", "conversation_id", conv.ID, "error", err)
	}
}

func (s *Service) scheduleUnanswered(conv *models.ChatConversation, seq int64) {
	if s.enqueuer == nil || s.cfg.UnansweredAfter <= 0 || conv.State == models.ChatStateSpam {
		return
	}
	if err := s.enqueuer.EnqueueChatUnanswered(conv.ID, seq, s.cfg.UnansweredAfter); err != nil {
		logger.Error("chat: failed to enqueue unanswered alert", "conversation_id", conv.ID, "error", err)
	}
}

func (s *Service) visitorQuotas(visitorID uint) []ratelimit.Quota {
	return []ratelimit.Quota{
		{Key: fmt.Sprintf("chat:visitor:msg:%d", visitorID), Limit: s.cfg.VisitorPerMinute, Window: time.Minute},
		{Key: fmt.Sprintf("chat:visitor:day:%d", visitorID), Limit: s.cfg.VisitorDaily, Window: 24 * time.Hour},
	}
}

func (s *Service) conversationQuota(conversationID uint) ratelimit.Quota {
	return ratelimit.Quota{
		Key: fmt.Sprintf("chat:conv:msg:%d", conversationID), Limit: s.cfg.ConversationPerMinute, Window: time.Minute,
	}
}

func (s *Service) check(ctx context.Context, quotas []ratelimit.Quota) error {
	if s.quotas == nil {
		return nil
	}
	if err := s.quotas.Check(ctx, quotas); err != nil {
		return ErrRateLimited
	}
	return nil
}

func (s *Service) touch(v *models.ChatVisitor, ip, page string) {
	if err := s.repo.TouchVisitor(v.ID, ip, page); err != nil {
		logger.Warn("chat: failed to update visitor", "visitor_id", v.ID, "error", err)
	}
}

func checkBody(raw string, limit int) (string, error) {
	body := CleanBody(raw)
	if body == "" {
		return "", ErrEmptyBody
	}
	if len(body) > limit {
		return "", ErrBodyTooLong
	}
	return body, nil
}

// Anything unlike a widget-generated ID is dropped, and the message is simply not idempotent.
func cleanClientID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" || len(id) > 36 {
		return ""
	}
	for _, r := range id {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' && r != '_' {
			return ""
		}
	}
	return id
}

// Lowercase base32 of 16 random bytes, the shape ParseChatThreadAddress accepts.
func newThreadToken() *string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return nil
	}
	token := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw))
	return &token
}

func preview(body string) string {
	return CleanLine(body, 140)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// lookup feeds the scanner the workspace-wide spam filters plus the widget's own.
type lookup struct {
	repo            *repositories.ChatRepository
	filterRepo      *repositories.MessageFilterRepository
	suppressionRepo *repositories.SuppressionRepository
}

func (l *lookup) IsSuppressed(workspaceID uint, email string) bool {
	if l.suppressionRepo == nil || email == "" {
		return false
	}
	ws := workspaceID
	suppressed, err := l.suppressionRepo.IsSuppressed(repositories.ResourceScope{WorkspaceID: &ws}, email)
	return err == nil && suppressed
}

func (l *lookup) SubmissionsFromIP(ip string, since time.Time) int64 {
	if l.repo == nil {
		return 0
	}
	return l.repo.CountVisitorStartsFromIP(ip, since)
}

func (l *lookup) SubmissionsFromSender(models.ScanSource, string, time.Time) int64 { return 0 }

func (l *lookup) ActiveFilters(workspaceID uint, src models.ScanSource) ([]models.MessageFilter, error) {
	if l.filterRepo == nil {
		return nil, nil
	}
	return l.filterRepo.FindActiveForWidget(workspaceID, src.ID)
}
