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

package handlers

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/goposta/posta/internal/dto"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/chat"
	"github.com/goposta/posta/internal/services/inbound"
	"github.com/goposta/posta/internal/services/messages"
	"github.com/goposta/posta/internal/services/realtime"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/goposta/posta/internal/web/widget"
	"github.com/jkaninda/logger"
	"github.com/jkaninda/okapi"
)

// ChatVisitorHandler is the widget's sessionless API; anything a visitor may not see is 404, never 403, to hide existence.
type ChatVisitorHandler struct {
	svc     *chat.Service
	repo    *repositories.ChatRepository
	limiter *inbound.IPRateLimiter
}

func NewChatVisitorHandler(svc *chat.Service, limiter *inbound.IPRateLimiter) *ChatVisitorHandler {
	return &ChatVisitorHandler{svc: svc, repo: svc.Repo(), limiter: limiter}
}

const widgetReasonOffline = "offline"

const (
	chatStreamPing     = 25 * time.Second
	chatReplayBatch    = 200
	chatRetryAfterSecs = "30"
)

type ChatWidgetConfig struct {
	Enabled         bool            `json:"enabled"`
	Reason          string          `json:"reason,omitempty" doc:"Why the widget cannot be used here: unavailable, origin_not_allowed, or offline (no one is online and the widget hides then)"`
	Online          bool            `json:"online" doc:"An agent is online and it is within business hours. Offline, the widget takes messages to answer by email."`
	Title           string          `json:"title,omitempty"`
	Greeting        string          `json:"greeting,omitempty"`
	AccentColor     string          `json:"accent_color,omitempty"`
	Position        string          `json:"position,omitempty"`
	AvatarURL       string          `json:"avatar_url,omitempty"`
	RequireName     bool            `json:"require_name,omitempty"`
	RequireEmail    bool            `json:"require_email,omitempty"`
	MaxMessageBytes int             `json:"max_message_bytes,omitempty"`
	Attachments     bool            `json:"attachments,omitempty" doc:"Visitors may send images and PDFs up to 5 MB"`
	MinFillSeconds  int             `json:"min_fill_seconds,omitempty" doc:"Seconds between issuing the nonce and starting a conversation with it"`
	Nonce           *messages.Nonce `json:"nonce,omitempty" doc:"Nonce for starting a conversation"`
}

type ChatKeyRequest struct {
	Key string `param:"key"`
}

type ChatStartRequest struct {
	Key  string `param:"key"`
	Body struct {
		Name        string `json:"name"`
		Email       string `json:"email"`
		Body        string `json:"body" required:"true"`
		ClientMsgID string `json:"client_msg_id"`
		Nonce       string `json:"nonce"`
		PageURL     string `json:"page_url"`
		// Identity is a signed identity from PostaChat.identify(); an invalid one leaves the visitor anonymous.
		Identity *chat.Identity `json:"identity"`
	} `json:"body"`
}

type ChatIdentifyRequest struct {
	Key  string        `param:"key"`
	Body chat.Identity `json:"body"`
}

type ChatIdentifyResponse struct {
	Verified     bool   `json:"verified"`
	VisitorToken string `json:"visitor_token,omitempty" doc:"Set when the call had no token and the identity verified: a new device of a known user gets a visitor, and with it their history"`
}

type ChatAttributesRequest struct {
	Key  string `param:"key"`
	Body struct {
		Attributes map[string]any `json:"attributes" doc:"Flat string, number or boolean values; at most 30, 2 KiB in all. Replaces the previous set."`
	} `json:"body"`
}

type ChatAttachmentRequest struct {
	Key string `param:"key"`
	ID  string `param:"id"`
	Msg string `param:"msg"`
	Idx int    `param:"idx"`
}

type ChatStartResponse struct {
	Status       string                    `json:"status" enum:"started,received" doc:"received: the message was accepted but no conversation follows"`
	VisitorToken string                    `json:"visitor_token,omitempty" doc:"Present the first time a visitor writes; send it as a bearer token from then on"`
	Conversation *chat.VisitorConversation `json:"conversation,omitempty"`
	Message      *chat.VisitorMessage      `json:"message,omitempty"`
}

type ChatVisitorConversationRequest struct {
	Key string `param:"key"`
	ID  string `param:"id"`
}

type ChatVisitorMessagesRequest struct {
	Key    string `param:"key"`
	ID     string `param:"id"`
	After  int64  `query:"after"`
	Before int64  `query:"before"`
	Limit  int    `query:"limit" default:"50"`
}

type ChatVisitorReadRequest struct {
	Key  string `param:"key"`
	ID   string `param:"id"`
	Body struct {
		Seq int64 `json:"seq" doc:"Newest message the visitor has seen"`
	} `json:"body"`
}

type ChatReadResponse struct {
	Seq int64 `json:"seq"`
}

type ChatVisitorPostRequest struct {
	Key  string `param:"key"`
	ID   string `param:"id"`
	Body struct {
		Body        string `json:"body" required:"true"`
		ClientMsgID string `json:"client_msg_id"`
		PageURL     string `json:"page_url"`
	} `json:"body"`
}

// Config always answers 200 so the widget can read why it should not render.
func (h *ChatVisitorHandler) Config(c *okapi.Context, req *ChatKeyRequest) error {
	c.ResponseWriter().Header().Set("Cache-Control", "no-store")
	w := h.widget(req.Key)
	if w == nil {
		return h.disabled(c, widgetReasonUnavailable)
	}
	if !h.allowOrigin(c, w) {
		return h.disabled(c, widgetReasonOriginNotAllowed)
	}
	ctx := c.Request().Context()
	online := h.svc.Online(ctx, w)
	if !online && w.HidesWhenOffline() {
		return h.disabled(c, widgetReasonOffline)
	}
	nonce, err := h.svc.Nonces().Issue(ctx, w.PublicKey)
	if err != nil {
		return c.AbortInternalServerError("failed to issue nonce")
	}
	greeting := w.Greeting
	if !online && w.OfflineGreeting != "" {
		greeting = w.OfflineGreeting
	}
	return ok(c, ChatWidgetConfig{
		Enabled:         true,
		Online:          online,
		Title:           w.Title,
		Greeting:        greeting,
		AccentColor:     w.AccentColor,
		Position:        w.Position,
		AvatarURL:       w.AvatarURL,
		RequireName:     w.RequireName,
		RequireEmail:    w.RequireEmail || !online,
		MaxMessageBytes: w.MessageLimit(),
		Attachments:     h.svc.AttachmentsEnabled(w),
		MinFillSeconds:  chat.MinFillSeconds,
		Nonce:           nonce,
	})
}

// disabled carries no widget data, so any origin may read it.
func (h *ChatVisitorHandler) disabled(c *okapi.Context, reason string) error {
	if origin := c.Request().Header.Get("Origin"); origin != "" {
		c.ResponseWriter().Header().Set("Access-Control-Allow-Origin", origin)
		c.ResponseWriter().Header().Add("Vary", "Origin")
	}
	return ok(c, ChatWidgetConfig{Reason: reason})
}

func (h *ChatVisitorHandler) Nonce(c *okapi.Context, req *ChatKeyRequest) error {
	if h.limiter != nil && !h.limiter.Allow(c.RealIP()) {
		return c.AbortTooManyRequests("too many requests")
	}
	w := h.widget(req.Key)
	if w == nil || !h.allowOrigin(c, w) {
		return c.AbortNotFound("chat not found")
	}
	nonce, err := h.svc.Nonces().Issue(c.Request().Context(), w.PublicKey)
	if err != nil {
		return c.AbortInternalServerError("failed to issue nonce")
	}
	return ok(c, nonce)
}

// Start joins a returning visitor's conversations when they send their token; a new visitor gets one back.
func (h *ChatVisitorHandler) Start(c *okapi.Context, req *ChatStartRequest) error {
	ip := c.RealIP()
	if h.limiter != nil && !h.limiter.Allow(ip) {
		return c.AbortTooManyRequests("too many conversations; try again later")
	}
	w := h.widget(req.Key)
	if w == nil || !h.allowOrigin(c, w) {
		return c.AbortNotFound("chat not found")
	}
	var visitor *models.ChatVisitor
	if c.Request().Header.Get("Authorization") != "" {
		if visitor = h.visitor(c, w); visitor == nil {
			return c.AbortUnauthorized("visitor token is not valid")
		}
	}
	b := req.Body
	res, err := h.svc.Start(c.Request().Context(), chat.StartRequest{
		Widget:      w,
		Visitor:     visitor,
		Name:        b.Name,
		Email:       b.Email,
		Body:        b.Body,
		ClientMsgID: b.ClientMsgID,
		Nonce:       b.Nonce,
		PageURL:     b.PageURL,
		ClientIP:    ip,
		UserAgent:   c.Request().UserAgent(),
		Identity:    b.Identity,
	})
	if err != nil {
		if isChatClientError(err) {
			return abortChat(c, err)
		}
		logger.Error("chat: failed to start conversation", "widget_id", w.ID, "error", err)
		return c.AbortInternalServerError("failed to start conversation")
	}
	if res.Rejected {
		return c.JSON(http.StatusAccepted, dto.Response[ChatStartResponse]{Success: true, Data: ChatStartResponse{Status: "received"}})
	}
	conv := chat.ToVisitorConversation(res.Conversation)
	msg := chat.ToVisitorMessage(res.Message, w.ShowAgentNames)
	return created(c, ChatStartResponse{
		Status:       "started",
		VisitorToken: res.Token,
		Conversation: &conv,
		Message:      &msg,
	})
}

func (h *ChatVisitorHandler) Conversations(c *okapi.Context, req *ChatKeyRequest) error {
	w := h.widget(req.Key)
	if w == nil || !h.allowOrigin(c, w) {
		return c.AbortNotFound("chat not found")
	}
	v := h.visitor(c, w)
	if v == nil {
		return c.AbortUnauthorized("visitor token is not valid")
	}
	items, err := h.repo.ListLinkedConversations(h.repo.LinkedVisitorIDs(v), 20)
	if err != nil {
		return c.AbortInternalServerError("failed to list conversations")
	}
	out := make([]chat.VisitorConversation, 0, len(items))
	for i := range items {
		out = append(out, chat.ToVisitorConversation(&items[i]))
	}
	return ok(c, out)
}

func (h *ChatVisitorHandler) Messages(c *okapi.Context, req *ChatVisitorMessagesRequest) error {
	w, _, conv, status := h.conversation(c, req.Key, req.ID)
	if status != 0 {
		return abortVisitor(c, status)
	}
	items, err := pageMessages(h.repo, conv, req.After, req.Before, req.Limit)
	if err != nil {
		return c.AbortInternalServerError("failed to load messages")
	}
	return ok(c, chat.ToVisitorMessages(items, w.ShowAgentNames))
}

func (h *ChatVisitorHandler) PostMessage(c *okapi.Context, req *ChatVisitorPostRequest) error {
	w, v, conv, status := h.conversation(c, req.Key, req.ID)
	if status != 0 {
		return abortVisitor(c, status)
	}
	msg, err := h.svc.PostVisitorMessage(c.Request().Context(), w, v, conv,
		req.Body.Body, req.Body.ClientMsgID, c.RealIP(), req.Body.PageURL)
	if err != nil {
		if isChatClientError(err) || errors.Is(err, repositories.ErrChatConversationGone) {
			return abortChat(c, err)
		}
		logger.Error("chat: failed to store visitor message", "conversation_id", conv.ID, "error", err)
		return c.AbortInternalServerError("failed to send message")
	}
	return created(c, chat.ToVisitorMessage(msg, w.ShowAgentNames))
}

// Identify answers 200 either way: verified=false means the signature did not match and nothing changed.
func (h *ChatVisitorHandler) Identify(c *okapi.Context, req *ChatIdentifyRequest) error {
	w := h.widget(req.Key)
	if w == nil || !h.allowOrigin(c, w) {
		return c.AbortNotFound("chat not found")
	}
	id := req.Body
	if c.Request().Header.Get("Authorization") == "" {
		if h.limiter != nil && !h.limiter.Allow(c.RealIP()) {
			return c.AbortTooManyRequests("too many requests")
		}
		v, token, err := h.svc.IdentifyNew(c.Request().Context(), w, &id, c.RealIP(), c.Request().UserAgent())
		if err != nil {
			return abortChat(c, err)
		}
		return ok(c, ChatIdentifyResponse{Verified: v != nil, VisitorToken: token})
	}
	v := h.visitor(c, w)
	if v == nil {
		return c.AbortUnauthorized("visitor token is not valid")
	}
	return ok(c, ChatIdentifyResponse{Verified: h.svc.Identify(w, v, &id)})
}

// Attributes replaces the custom attributes the page set on the visitor.
func (h *ChatVisitorHandler) Attributes(c *okapi.Context, req *ChatAttributesRequest) error {
	w := h.widget(req.Key)
	if w == nil || !h.allowOrigin(c, w) {
		return c.AbortNotFound("chat not found")
	}
	v := h.visitor(c, w)
	if v == nil {
		return c.AbortUnauthorized("visitor token is not valid")
	}
	if err := h.svc.SetAttributes(v, req.Body.Attributes); err != nil {
		if errors.Is(err, chat.ErrInvalidAttributes) {
			return c.AbortBadRequest(err.Error())
		}
		return c.AbortInternalServerError("failed to store attributes")
	}
	return noContent(c)
}

// Upload takes a multipart file part plus optional caption and client_msg_id parts.
func (h *ChatVisitorHandler) Upload(c *okapi.Context) error {
	w, v, conv, status := h.conversation(c, c.Param("key"), c.Param("id"))
	if status != 0 {
		return abortVisitor(c, status)
	}
	up, err := readUpload(c)
	if err != nil {
		return abortChat(c, err)
	}
	msg, err := h.svc.PostVisitorFile(c.Request().Context(), w, v, conv, up, c.RealIP())
	if err != nil {
		if isChatClientError(err) {
			return abortChat(c, err)
		}
		logger.Error("chat: failed to store visitor file", "conversation_id", conv.ID, "error", err)
		return c.AbortInternalServerError("failed to send file")
	}
	return created(c, chat.ToVisitorMessage(msg, w.ShowAgentNames))
}

func (h *ChatVisitorHandler) Attachment(c *okapi.Context, req *ChatAttachmentRequest) error {
	_, _, conv, status := h.conversation(c, req.Key, req.ID)
	if status != 0 {
		return abortVisitor(c, status)
	}
	return serveAttachment(c, h.svc, h.repo, conv, req.Msg, req.Idx)
}

// Typing is throttled and stores nothing.
func (h *ChatVisitorHandler) Typing(c *okapi.Context, req *ChatVisitorConversationRequest) error {
	_, _, conv, status := h.conversation(c, req.Key, req.ID)
	if status != 0 {
		return abortVisitor(c, status)
	}
	h.svc.Typing(c.Request().Context(), conv, true, "")
	return noContent(c)
}

func (h *ChatVisitorHandler) Read(c *okapi.Context, req *ChatVisitorReadRequest) error {
	_, _, conv, status := h.conversation(c, req.Key, req.ID)
	if status != 0 {
		return abortVisitor(c, status)
	}
	seq, err := h.svc.MarkRead(c.Request().Context(), conv, true, req.Body.Seq)
	if err != nil {
		return c.AbortInternalServerError("failed to record read")
	}
	return ok(c, ChatReadResponse{Seq: seq})
}

// Stream replays what the visitor missed, then relays live events, using seqs to drop duplicates and refetch losses.
func (h *ChatVisitorHandler) Stream(c *okapi.Context) error {
	w, v, conv, status := h.conversation(c, c.Param("key"), c.Param("id"))
	if status != 0 {
		return abortVisitor(c, status)
	}
	bus := h.svc.Bus()
	if bus == nil || !h.svc.AcquireStream() {
		c.ResponseWriter().Header().Set("Retry-After", chatRetryAfterSecs)
		return c.AbortServiceUnavailable("too many open chats; poll for messages instead")
	}
	defer h.svc.ReleaseStream()

	cursor := chat.Cursor{Last: resumeSeq(c.Request())}
	ctx := c.Request().Context()
	// Subscribe before reading the database so nothing stored in between is missed.
	events, unsub := bus.Subscribe(ctx, realtime.ChatConversationTopic(conv.ID))
	defer unsub()
	_ = h.repo.TouchVisitor(v.ID, c.RealIP(), "")
	c.ResponseWriter().Header().Set("X-Accel-Buffering", "no")
	// An open stream counts as presence, so agents' replies stay in the chat instead of going out by email.
	if h.svc.MarkVisitorLive(ctx, conv.ID) {
		h.svc.AnnounceVisitor(ctx, conv, true)
	}
	defer func() {
		bg := context.Background()
		h.svc.ClearVisitorLive(bg, conv.ID)
		_ = h.repo.TouchVisitor(v.ID, c.RealIP(), "")
		h.svc.AnnounceVisitor(bg, conv, false)
	}()
	keepAlive := time.NewTicker(chatStreamPing)
	defer keepAlive.Stop()

	out := make(chan okapi.Message, 16)
	go func() {
		defer close(out)
		send := func(m okapi.Message) bool {
			select {
			case out <- m:
				return true
			case <-ctx.Done():
				return false
			}
		}
		if !send(okapi.Message{
			ID: strconv.FormatInt(cursor.Last, 10), Event: chat.EventConversation, Data: chat.ToVisitorConversation(conv),
		}) {
			return
		}
		if !h.replay(ctx, conv.ID, &cursor, w.ShowAgentNames, send) {
			return
		}
		for {
			select {
			case <-ctx.Done():
				return
			case <-keepAlive.C:
				// Another tab's stream may have closed and cleared the mark.
				if h.svc.MarkVisitorLive(ctx, conv.ID) {
					h.svc.AnnounceVisitor(ctx, conv, true)
				}
			case ev, open := <-events:
				if !open {
					return
				}
				if ev.WorkspaceID != *conv.WorkspaceID {
					continue
				}
				switch cursor.Admit(ev.Seq) {
				case chat.Skip:
					continue
				case chat.Refetch:
					if !h.replay(ctx, conv.ID, &cursor, w.ShowAgentNames, send) {
						return
					}
					continue
				}
				// Each event carries the visitor's newest seq so Last-Event-ID always resumes in the right place.
				if !send(okapi.Message{ID: strconv.FormatInt(cursor.Last, 10), Event: ev.Type, Data: ev.Data}) {
					return
				}
			}
		}
	}()
	return c.SSEStreamWithOptions(ctx, out, &okapi.StreamOptions{
		Serializer:   &okapi.JSONSerializer{},
		PingInterval: chatStreamPing,
	})
}

func (h *ChatVisitorHandler) replay(ctx context.Context, convID uint, cursor *chat.Cursor, showNames bool, send func(okapi.Message) bool) bool {
	for {
		items, err := h.repo.MessagesAfter(convID, cursor.Last, chatReplayBatch)
		if err != nil {
			logger.Warn("chat: replay failed", "conversation_id", convID, "error", err)
			return ctx.Err() == nil
		}
		for i := range items {
			m := chat.ToVisitorMessage(&items[i], showNames)
			cursor.Advance(m.Seq)
			if !send(okapi.Message{ID: strconv.FormatInt(m.Seq, 10), Event: chat.EventMessage, Data: m}) {
				return false
			}
		}
		if len(items) < chatReplayBatch {
			return true
		}
	}
}

func (h *ChatVisitorHandler) ChatScript(c *okapi.Context) error {
	w := c.ResponseWriter()
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("ETag", widget.ChatScriptETag)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
	http.ServeContent(w, c.Request(), "posta-chat.js", time.Time{}, bytes.NewReader(widget.ChatScript))
	return nil
}

// Preflight answers plain OPTIONS only; CORS preflights are answered earlier by config.ChatPreflight.
func (h *ChatVisitorHandler) Preflight(c *okapi.Context) error {
	return c.NoContent()
}

// widget treats unknown, paused and archived widgets alike as absent.
func (h *ChatVisitorHandler) widget(key string) *models.ChatWidget {
	if !strings.HasPrefix(key, models.ChatWidgetKeyPrefix) {
		return nil
	}
	w, err := h.repo.FindWidgetByKey(key)
	if err != nil || !w.IsActive() || w.WorkspaceID == nil {
		return nil
	}
	return w
}

// allowOrigin never allows credentials: the chat API uses no cookies.
func (h *ChatVisitorHandler) allowOrigin(c *okapi.Context, w *models.ChatWidget) bool {
	origin := c.Request().Header.Get("Origin")
	allowlist, strict := w.OriginPolicy()
	if !messages.OriginAllowed(allowlist, strict, origin) {
		return false
	}
	if origin != "" {
		hdr := c.ResponseWriter().Header()
		hdr.Set("Access-Control-Allow-Origin", origin)
		hdr.Del("Access-Control-Allow-Credentials")
		hdr.Add("Vary", "Origin")
	}
	return true
}

func (h *ChatVisitorHandler) visitor(c *okapi.Context, w *models.ChatWidget) *models.ChatVisitor {
	token := chat.BearerVisitorToken(c.Request().Header.Get("Authorization"))
	if token == "" {
		return nil
	}
	v, err := h.repo.FindVisitorByTokenHash(chat.HashVisitorToken(token))
	if err != nil || v.WidgetID != w.ID {
		return nil
	}
	return v
}

func (h *ChatVisitorHandler) conversation(c *okapi.Context, key, id string) (*models.ChatWidget, *models.ChatVisitor, *models.ChatConversation, int) {
	w := h.widget(key)
	if w == nil || !h.allowOrigin(c, w) {
		return nil, nil, nil, http.StatusNotFound
	}
	v := h.visitor(c, w)
	if v == nil {
		return nil, nil, nil, http.StatusUnauthorized
	}
	// A verified visitor also reaches the same customer user's conversations from other devices.
	conv, err := h.repo.FindLinkedConversation(h.repo.LinkedVisitorIDs(v), id)
	if err != nil {
		return nil, nil, nil, http.StatusNotFound
	}
	return w, v, conv, 0
}

func abortVisitor(c *okapi.Context, status int) error {
	if status == http.StatusUnauthorized {
		return c.AbortUnauthorized("visitor token is not valid")
	}
	return c.AbortNotFound("conversation not found")
}

func isChatClientError(err error) bool {
	for _, e := range []error{chat.ErrEmptyBody, chat.ErrBodyTooLong, chat.ErrNameRequired,
		chat.ErrEmailRequired, chat.ErrInvalidEmail, chat.ErrRateLimited,
		chat.ErrAttachmentsOff, chat.ErrAttachmentTooLarge, chat.ErrAttachmentType, chat.ErrInvalidAttributes} {
		if errors.Is(err, e) {
			return true
		}
	}
	return false
}

// resumeSeq reads Last-Event-ID, which browsers send on reconnect, or ?after= on the first connect.
func resumeSeq(r *http.Request) int64 {
	for _, raw := range []string{r.Header.Get("Last-Event-ID"), r.URL.Query().Get("after")} {
		if n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64); err == nil && n > 0 {
			return n
		}
	}
	return 0
}

// readUpload refuses anything over the cap before it is read into memory.
func readUpload(c *okapi.Context) (chat.Upload, error) {
	r := c.Request()
	r.Body = http.MaxBytesReader(c.ResponseWriter(), r.Body, chat.MaxAttachmentBytes+64<<10)
	if err := r.ParseMultipartForm(chat.MaxAttachmentBytes + 64<<10); err != nil {
		return chat.Upload{}, chat.ErrAttachmentTooLarge
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		return chat.Upload{}, chat.ErrAttachmentType
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, chat.MaxAttachmentBytes+1))
	if err != nil {
		return chat.Upload{}, err
	}
	if len(data) > chat.MaxAttachmentBytes {
		return chat.Upload{}, chat.ErrAttachmentTooLarge
	}
	return chat.Upload{
		Filename:    header.Filename,
		Data:        data,
		Caption:     r.FormValue("caption"),
		ClientMsgID: r.FormValue("client_msg_id"),
	}, nil
}

// serveAttachment sets nosniff and a sandbox CSP, and downloads PDFs, so nothing renders on Posta's origin.
func serveAttachment(c *okapi.Context, svc *chat.Service, repo *repositories.ChatRepository, conv *models.ChatConversation, msgID string, idx int) error {
	msg, err := repo.FindMessage(conv.ID, msgID)
	if err != nil {
		return c.AbortNotFound("file not found")
	}
	body, att, err := svc.OpenAttachment(c.Request().Context(), msg, idx)
	if err != nil {
		return c.AbortNotFound("file not found")
	}
	defer func() { _ = body.Close() }()
	hdr := c.ResponseWriter().Header()
	hdr.Set("Content-Type", att.ContentType)
	hdr.Set("X-Content-Type-Options", "nosniff")
	hdr.Set("Content-Security-Policy", "sandbox; default-src 'none'; img-src 'self'")
	hdr.Set("Cache-Control", "private, max-age=3600")
	disposition := "inline"
	if !strings.HasPrefix(att.ContentType, "image/") {
		disposition = dispositionAttachment
	}
	hdr.Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": att.Filename}))
	c.ResponseWriter().WriteHeader(http.StatusOK)
	_, err = io.Copy(c.ResponseWriter(), body)
	return err
}
