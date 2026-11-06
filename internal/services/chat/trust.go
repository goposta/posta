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

// MaxAttachmentBytes caps one chat attachment.
const MaxAttachmentBytes = 5 << 20

// spamHitsToSpam is how many rejected later messages turn a conversation
// into spam.
const spamHitsToSpam = 3

// banTTL is how long a banned visitor's IP stays blocked.
const banTTL = 24 * time.Hour

// Identity is what PostaChat.identify() sends: the customer's user ID and
// email, signed on the customer's server.
type Identity struct {
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Signature string `json:"signature" doc:"Hex HMAC-SHA256 of user_id + \".\" + email, keyed with the widget's identity secret"`
}

// SignIdentity is the signature a customer's server computes for an identity.
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

// VerifyIdentity reports whether id carries a valid signature for the
// widget. A widget without a secret verifies nothing; a rotated secret makes
// old signatures fail, and their visitors simply stay anonymous.
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

// Identify applies a verified identity to a visitor: their email becomes
// verified, and they share history with the same customer user on other
// devices. An invalid identity changes nothing.
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

// IdentifyNew creates a visitor from a verified identity, so the same person
// on a new device sees their history before writing. It returns the visitor
// and its token, or nil when the identity does not verify.
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

// SetAttributes replaces a visitor's custom attributes.
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

// CleanAttributes keeps flat values only and enforces the size cap.
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

// Ban stops a visitor: their messages are accepted and thrown away, and
// their IP cannot start a new chat in the workspace for a day.
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

// Unban lifts a ban.
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

// ipBanned reports whether ip is blocked in the workspace.
func (s *Service) ipBanned(ctx context.Context, workspaceID uint, ip string) bool {
	if s.rdb == nil || ip == "" {
		return false
	}
	n, err := s.rdb.Exists(ctx, banKey(workspaceID, ip)).Result()
	return err == nil && n > 0
}

// discarded is the answer to a message Posta drops without saying so: it
// looks stored to the sender, and is not.
func discarded(body, clientMsgID string) *models.ChatMessage {
	return &models.ChatMessage{
		UUID: uuid.NewString(), AuthorKind: models.ChatAuthorVisitor,
		Body: body, ClientMsgID: clientMsgID, CreatedAt: time.Now(),
	}
}

// scanLater runs the light scan on a later visitor message. It reports
// whether to drop the message, and counts the hit; the third makes the
// conversation spam.
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

// SetBlobStore enables attachments.
func (s *Service) SetBlobStore(bs blob.Store) { s.blobs = bs }

// AttachmentsEnabled reports whether the widget can take files here.
func (s *Service) AttachmentsEnabled(w *models.ChatWidget) bool {
	return w.AllowAttachments && s.blobs != nil
}

// allowedAttachmentTypes are sniffed from the bytes, never taken from the
// client's claim.
var allowedAttachmentTypes = map[string]string{
	"image/png":       ".png",
	"image/jpeg":      ".jpg",
	"image/gif":       ".gif",
	"image/webp":      ".webp",
	"application/pdf": ".pdf",
}

// Upload is a file sent in the chat with an optional caption.
type Upload struct {
	Filename    string
	Data        []byte
	Caption     string
	ClientMsgID string
}

// storeAttachment checks and stores a file and returns its record.
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

// PostVisitorFile adds a visitor's file to their conversation.
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

// PostAgentFile adds an agent's file to a conversation.
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

// OpenAttachment returns one of a message's files.
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

// SetDirectory lets auto-assign see the workspace's members.
func (s *Service) SetDirectory(ws *repositories.WorkspaceRepository) { s.workspaces = ws }

// autoAssign gives a new conversation to the next online agent in turn,
// skipping those at the widget's cap. Nobody online, nobody assigned.
func (s *Service) autoAssign(ctx context.Context, w *models.ChatWidget, conv *models.ChatConversation) {
	// Auto-assign is Posta Enterprise. Without a license, chats wait to be
	// claimed, as they do with auto-assign off.
	if !w.AutoAssign || s.licensed == nil || !s.licensed() || s.workspaces == nil || s.rdb == nil || conv.WorkspaceID == nil {
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

// Erase hard-deletes a conversation, its messages and files, and its
// visitor when it was their last.
func (s *Service) Erase(ctx context.Context, conv *models.ChatConversation) error {
	keys, err := s.repo.EraseConversation(conv.ID)
	if err != nil {
		return err
	}
	s.DeleteBlobs(ctx, keys)
	return nil
}

// DeleteBlobs removes attachment files after their records are gone; a
// failed delete is logged, never retried into a half-erased record.
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
