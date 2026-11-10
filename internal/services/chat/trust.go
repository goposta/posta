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
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/crypto"
	"github.com/goposta/posta/internal/services/messagescan"
	"github.com/goposta/posta/internal/services/ratelimit"
	"github.com/goposta/posta/internal/storage/blob"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/logger"
)

var (
	ErrAttachmentsOff     = errors.New("attachments are not enabled for this chat")
	ErrAttachmentTooLarge = errors.New("attachments can be at most 5 MB")
	ErrAttachmentType     = errors.New("only images (PNG, JPEG, GIF, WebP) and PDF files can be sent")
	ErrInvalidAttributes  = errors.New("attributes must be at most 30 flat string, number or boolean values, 2 KiB in all")
)

const MaxAttachmentBytes = 5 << 20

// spamHitsToSpam is how many rejected later messages turn a conversation into spam.
const spamHitsToSpam = 3

const banTTL = 24 * time.Hour

// Identity is what PostaChat.identify() sends, signed on the customer's server.
type Identity struct {
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Signature string `json:"signature" doc:"Hex HMAC-SHA256 of user_id + \".\" + email, keyed with the widget's identity secret"`
}

func SignIdentity(secret, userID, email string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(userID + "." + strings.ToLower(strings.TrimSpace(email))))
	return hex.EncodeToString(mac.Sum(nil))
}

// NewIdentitySecret returns a fresh secret and its stored, encrypted form.
func NewIdentitySecret() (secret, stored string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	secret = "pcs_" + hex.EncodeToString(raw)
	stored, err = crypto.Encrypt(secret)
	return secret, stored, err
}

// A rotated or missing secret makes signatures fail, and those visitors simply stay anonymous.
func VerifyIdentity(w *models.ChatWidget, id *Identity) bool {
	if id == nil || w.IdentitySecretEnc == "" || id.UserID == "" || id.Email == "" || len(id.UserID) > 128 {
		return false
	}
	secret, err := crypto.Decrypt(w.IdentitySecretEnc)
	if err != nil || secret == "" {
		return false
	}
	want := SignIdentity(secret, id.UserID, id.Email)
	return hmac.Equal([]byte(want), []byte(strings.ToLower(strings.TrimSpace(id.Signature))))
}

// A verified identity marks the email verified and shares history with the same customer user on other devices.
func (s *Service) Identify(w *models.ChatWidget, v *models.ChatVisitor, id *Identity) bool {
	if !VerifyIdentity(w, id) {
		return false
	}
	email := strings.ToLower(strings.TrimSpace(id.Email))
	fields := map[string]any{"external_id": id.UserID, "email": email, "email_verified": true}
	if name := CleanLine(id.Name, 120); name != "" {
		fields["name"] = name
		v.Name = name
	}
	if err := s.repo.UpdateVisitor(v.ID, fields); err != nil {
		logger.Warn("chat: failed to store identity", "visitor_id", v.ID, "error", err)
		return false
	}
	v.ExternalID, v.Email, v.EmailVerified = id.UserID, email, true
	return true
}

// IdentifyNew lets the same person on a new device see their history before writing; nil when it does not verify.
func (s *Service) IdentifyNew(ctx context.Context, w *models.ChatWidget, id *Identity, ip, userAgent string) (*models.ChatVisitor, string, error) {
	if !VerifyIdentity(w, id) {
		return nil, "", nil
	}
	if err := s.check(ctx, []ratelimit.Quota{{
		Key: fmt.Sprintf("chat:ip:identify:%d:%s", w.ID, ip), Limit: 30, Window: time.Hour,
	}}); err != nil {
		return nil, "", err
	}
	token, hash, err := NewVisitorToken()
	if err != nil {
		return nil, "", err
	}
	v := &models.ChatVisitor{
		WorkspaceID: w.WorkspaceID, WidgetID: w.ID, TokenHash: hash,
		FirstSeenIP: ip, LastSeenIP: ip, UserAgent: truncate(userAgent, 500), LastSeenAt: time.Now(),
	}
	if err := s.repo.CreateVisitor(v); err != nil {
		return nil, "", err
	}
	s.Identify(w, v, id)
	return v, token, nil
}

func (s *Service) SetAttributes(v *models.ChatVisitor, attrs map[string]any) error {
	clean, err := CleanAttributes(attrs)
	if err != nil {
		return err
	}
	if err := s.repo.UpdateVisitor(v.ID, map[string]any{"attributes": clean}); err != nil {
		return err
	}
	v.Attributes = clean
	return nil
}

func CleanAttributes(attrs map[string]any) (models.ChatAttributes, error) {
	if len(attrs) > 30 {
		return nil, ErrInvalidAttributes
	}
	out := models.ChatAttributes{}
	for k, v := range attrs {
		k = CleanLine(k, 40)
		if k == "" {
			return nil, ErrInvalidAttributes
		}
		switch t := v.(type) {
		case string:
			out[k] = CleanLine(t, 200)
		case float64, bool:
			out[k] = t
		case nil:
		default:
			return nil, ErrInvalidAttributes
		}
	}
	encoded, _ := json.Marshal(out)
	if len(encoded) > models.MaxChatAttributesBytes {
		return nil, ErrInvalidAttributes
	}
	return out, nil
}

func banKey(workspaceID uint, ip string) string {
	return fmt.Sprintf("chat:ban:%d:%s", workspaceID, ip)
}

// Ban accepts and discards a visitor's messages, and blocks their IP from new chats in the workspace for a day.
func (s *Service) Ban(ctx context.Context, conv *models.ChatConversation) error {
	v, err := s.repo.FindVisitorByID(conv.VisitorID)
	if err != nil {
		return err
	}
	now := time.Now()
	if err := s.repo.UpdateVisitor(v.ID, map[string]any{"banned_at": now}); err != nil {
		return err
	}
	if s.rdb != nil && conv.WorkspaceID != nil {
		for _, ip := range []string{v.FirstSeenIP, v.LastSeenIP} {
			if ip != "" {
				s.rdb.Set(ctx, banKey(*conv.WorkspaceID, ip), 1, banTTL)
			}
		}
	}
	return s.SetState(ctx, conv, models.ChatStateSpam)
}

func (s *Service) Unban(ctx context.Context, conv *models.ChatConversation) error {
	v, err := s.repo.FindVisitorByID(conv.VisitorID)
	if err != nil {
		return err
	}
	if err := s.repo.UpdateVisitor(v.ID, map[string]any{"banned_at": nil}); err != nil {
		return err
	}
	if s.rdb != nil && conv.WorkspaceID != nil {
		s.rdb.Del(ctx, banKey(*conv.WorkspaceID, v.FirstSeenIP), banKey(*conv.WorkspaceID, v.LastSeenIP))
	}
	return nil
}

func (s *Service) ipBanned(ctx context.Context, workspaceID uint, ip string) bool {
	if s.rdb == nil || ip == "" {
		return false
	}
	n, err := s.rdb.Exists(ctx, banKey(workspaceID, ip)).Result()
	return err == nil && n > 0
}

// discarded looks stored to the sender but is not, so a dropped message reveals nothing.
func discarded(body, clientMsgID string) *models.ChatMessage {
	return &models.ChatMessage{
		UUID: uuid.NewString(), AuthorKind: models.ChatAuthorVisitor,
		Body: body, ClientMsgID: clientMsgID, CreatedAt: time.Now(),
	}
}

// scanLater counts each hit; the third makes the conversation spam.
func (s *Service) scanLater(ctx context.Context, w *models.ChatWidget, conv *models.ChatConversation, body string) bool {
	verdict := s.scanner.Scan(messagescan.Input{Policy: w, WorkspaceID: *w.WorkspaceID, Body: body, Light: true})
	if verdict.Action != messagescan.ActionReject {
		return false
	}
	hits, err := s.repo.AddSpamHit(conv.ID)
	if err == nil && hits >= spamHitsToSpam && conv.State != models.ChatStateSpam {
		if err := s.SetState(ctx, conv, models.ChatStateSpam); err != nil {
			logger.Warn("chat: failed to mark conversation spam", "conversation_id", conv.ID, "error", err)
		}
	}
	return true
}

func (s *Service) SetBlobStore(bs blob.Store) { s.blobs = bs }

func (s *Service) AttachmentsEnabled(w *models.ChatWidget) bool {
	return w.AllowAttachments && s.blobs != nil
}

// Types are sniffed from the bytes, never taken from the client's claim.
var allowedAttachmentTypes = map[string]string{
	"image/png":       ".png",
	"image/jpeg":      ".jpg",
	"image/gif":       ".gif",
	"image/webp":      ".webp",
	"application/pdf": ".pdf",
}

type Upload struct {
	Filename    string
	Data        []byte
	Caption     string
	ClientMsgID string
}

func (s *Service) storeAttachment(ctx context.Context, w *models.ChatWidget, conv *models.ChatConversation, up Upload) (models.ChatAttachment, error) {
	if !s.AttachmentsEnabled(w) {
		return models.ChatAttachment{}, ErrAttachmentsOff
	}
	if len(up.Data) == 0 || len(up.Data) > MaxAttachmentBytes {
		return models.ChatAttachment{}, ErrAttachmentTooLarge
	}
	contentType := http.DetectContentType(up.Data)
	if i := strings.IndexByte(contentType, ';'); i >= 0 {
		contentType = contentType[:i]
	}
	ext, ok := allowedAttachmentTypes[contentType]
	if !ok {
		return models.ChatAttachment{}, ErrAttachmentType
	}
	name := CleanLine(path.Base(strings.ReplaceAll(up.Filename, "\\", "/")), 120)
	if name == "" || name == "." || name == "/" {
		name = "file" + ext
	}
	key := fmt.Sprintf("chat/%d/%s/%s%s", *conv.WorkspaceID, conv.UUID, uuid.NewString(), ext)
	if err := s.blobs.Put(ctx, key, bytes.NewReader(up.Data), contentType); err != nil {
		return models.ChatAttachment{}, err
	}
	return models.ChatAttachment{Filename: name, ContentType: contentType, Size: int64(len(up.Data)), Key: key}, nil
}

func (s *Service) PostVisitorFile(ctx context.Context, w *models.ChatWidget, v *models.ChatVisitor, conv *models.ChatConversation, up Upload, ip string) (*models.ChatMessage, error) {
	caption := CleanBody(up.Caption)
	if len(caption) > w.MessageLimit() {
		return nil, ErrBodyTooLong
	}
	if v.BannedAt != nil {
		return discarded(caption, up.ClientMsgID), nil
	}
	if err := s.check(ctx, append(s.visitorQuotas(v.ID), s.conversationQuota(conv.ID))); err != nil {
		return nil, err
	}
	att, err := s.storeAttachment(ctx, w, conv, up)
	if err != nil {
		return nil, err
	}
	msg := &models.ChatMessage{
		WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID, AuthorKind: models.ChatAuthorVisitor,
		Body: caption, ClientMsgID: cleanClientID(up.ClientMsgID), Attachments: models.ChatAttachments{att},
	}
	return s.appendVisitor(ctx, w, v, conv, msg, ip, "")
}

func (s *Service) PostAgentFile(ctx context.Context, conv *models.ChatConversation, agent Agent, up Upload) (*models.ChatMessage, error) {
	w, err := s.repo.FindWidgetByID(conv.WidgetID)
	if err != nil {
		return nil, err
	}
	caption := CleanBody(up.Caption)
	if len(caption) > w.MessageLimit() {
		return nil, ErrBodyTooLong
	}
	att, err := s.storeAttachment(ctx, w, conv, up)
	if err != nil {
		return nil, err
	}
	return s.appendAgent(ctx, w, conv, agent, &models.ChatMessage{
		WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID, AuthorKind: models.ChatAuthorAgent,
		AuthorID: &agent.ID, Body: caption, ClientMsgID: cleanClientID(up.ClientMsgID),
		Attachments: models.ChatAttachments{att},
	})
}

func (s *Service) OpenAttachment(ctx context.Context, msg *models.ChatMessage, idx int) (io.ReadCloser, *models.ChatAttachment, error) {
	if s.blobs == nil || idx < 0 || idx >= len(msg.Attachments) {
		return nil, nil, ErrAttachmentsOff
	}
	att := msg.Attachments[idx]
	r, err := s.blobs.Get(ctx, att.Key)
	if err != nil {
		return nil, nil, err
	}
	return r, &att, nil
}

func (s *Service) SetDirectory(ws *repositories.WorkspaceRepository) { s.workspaces = ws }

// autoAssign takes online agents in turn, skipping those at the widget's cap; nobody online, nobody assigned.
func (s *Service) autoAssign(ctx context.Context, w *models.ChatWidget, conv *models.ChatConversation) {
	// Auto-assign is Enterprise; unlicensed chats wait to be claimed, as with auto-assign off.
	if !w.AutoAssign || !s.Licensed() || s.workspaces == nil || s.rdb == nil || conv.WorkspaceID == nil {
		return
	}
	ws := *conv.WorkspaceID
	online, err := s.presence.List(ctx, ws)
	if err != nil || len(online) == 0 {
		return
	}
	members, err := s.workspaces.ListMembers(ws)
	if err != nil {
		return
	}
	counts, _ := s.repo.OpenAssignedCounts(ws)
	var candidates []uint
	for _, m := range members {
		if online[m.UserID] != StatusOnline || !m.Role.CanEdit() {
			continue
		}
		if w.MaxOpenPerAgent > 0 && counts[m.UserID] >= w.MaxOpenPerAgent {
			continue
		}
		candidates = append(candidates, m.UserID)
	}
	if len(candidates) == 0 {
		return
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i] < candidates[j] })
	turn, err := s.rdb.Incr(ctx, fmt.Sprintf("chat:rr:%d", w.ID)).Result()
	if err != nil {
		return
	}
	pick := candidates[int((turn-1)%int64(len(candidates)))]
	if err := s.repo.UpdateConversationFields(conv.ID, map[string]any{"assigned_to_id": pick}); err == nil {
		conv.AssignedToID = &pick
	}
}

// Erase also deletes the visitor when this was their last conversation.
func (s *Service) Erase(ctx context.Context, conv *models.ChatConversation) error {
	keys, err := s.repo.EraseConversation(conv.ID)
	if err != nil {
		return err
	}
	s.DeleteBlobs(ctx, keys)
	return nil
}

// A failed blob delete is logged, never retried into a half-erased record.
func (s *Service) DeleteBlobs(ctx context.Context, keys []string) {
	if s.blobs == nil {
		return
	}
	for _, key := range keys {
		if err := s.blobs.Delete(ctx, key); err != nil {
			logger.Warn("chat: failed to delete attachment", "key", key, "error", err)
		}
	}
}
