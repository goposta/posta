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
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/cache"
	"github.com/goposta/posta/internal/services/email"
	"github.com/goposta/posta/internal/services/messagescan"
	"github.com/goposta/posta/internal/services/ratelimit"
	"github.com/goposta/posta/internal/storage/blob"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/logger"
	"github.com/redis/go-redis/v9"
)

var (
	ErrFormNotFound     = errors.New("form not found")
	ErrFormInactive     = errors.New("form is not accepting submissions")
	ErrOriginNotAllowed = errors.New("origin not allowed")
	ErrPayloadTooLarge  = errors.New("payload too large")
	ErrTooManyFields    = errors.New("too many fields")
	ErrNoRecipient      = errors.New("message has no reply address")
	ErrSenderNotSet     = errors.New("no reply sender configured for this form")
)

const dedupWindow = 5 * time.Minute

type Config struct {
	PerIPHourly       int
	PerFormHourly     int
	PerEmailHourly    int
	PerWorkspaceDaily int
	MaxBodyBytes      int64
	MaxAttachmentSize int64
	InboundDomain     string
	AppWebURL         string
}

type Enqueuer interface {
	EnqueueMessageProcess(messageID uint) error
	EnqueueMessageReply(replyID uint) error
}

type Service struct {
	formRepo        *repositories.FormRepository
	messageRepo     *repositories.MessageRepository
	filterRepo      *repositories.MessageFilterRepository
	suppressionRepo *repositories.SuppressionRepository
	emailService    *email.Service
	scanner         *messagescan.Scanner
	redis           *redis.Client
	quotas          quotaChecker
	formCache       formKeyCache
	blobStore       blob.Store
	enqueuer        Enqueuer
	cfg             Config
	hmacKey         []byte

	onReceived func(status models.MessageStatus)
}

func NewService(
	formRepo *repositories.FormRepository,
	messageRepo *repositories.MessageRepository,
	filterRepo *repositories.MessageFilterRepository,
	suppressionRepo *repositories.SuppressionRepository,
	redisClient *redis.Client,
	cfg Config,
	hmacKey []byte,
) *Service {
	s := &Service{
		formRepo:        formRepo,
		messageRepo:     messageRepo,
		filterRepo:      filterRepo,
		suppressionRepo: suppressionRepo,
		redis:           redisClient,
		cfg:             cfg,
		hmacKey:         hmacKey,
	}
	if redisClient != nil {
		s.quotas = ratelimit.NewQuotas(redisClient)
		s.formCache = cache.New(redisClient)
	}
	s.scanner = messagescan.New(&lookup{
		messageRepo:     messageRepo,
		filterRepo:      filterRepo,
		suppressionRepo: suppressionRepo,
	})
	return s
}

func (s *Service) SetEmailService(es *email.Service)        { s.emailService = es }
func (s *Service) SetBlobStore(bs blob.Store)               { s.blobStore = bs }
func (s *Service) SetEnqueuer(e Enqueuer)                   { s.enqueuer = e }
func (s *Service) OnReceived(fn func(models.MessageStatus)) { s.onReceived = fn }

func (s *Service) Scanner() *messagescan.Scanner { return s.scanner }

// OriginAllowed reports whether a browser Origin may post to an endpoint with
// this allowlist. An empty allowlist accepts any origin; "*" does explicitly.
// A missing Origin (curl, server-to-server, a native app) is accepted unless
// strict is set. The allowlist slows spam; it is not authentication.
func OriginAllowed(allowlist []string, strict bool, origin string) bool {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return !strict
	}
	if len(allowlist) == 0 {
		return true
	}
	for _, allowed := range allowlist {
		allowed = strings.TrimSpace(allowed)
		if allowed == "*" {
			return true
		}
		if strings.EqualFold(strings.TrimSuffix(allowed, "/"), strings.TrimSuffix(origin, "/")) {
			return true
		}
	}
	return false
}

// RedirectAllowed reports whether target is a safe post-submit redirect: on
// the host of the endpoint's configured redirect, or on an explicitly allowed
// origin. Anything else would be an open redirect on a trusted domain.
func RedirectAllowed(allowlist []string, fixedRedirect, target string) bool {
	if target == "" {
		return false
	}
	u, err := url.Parse(target)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return false
	}
	candidate := u.Scheme + "://" + u.Host
	if fixedRedirect != "" {
		if ru, err := url.Parse(fixedRedirect); err == nil && strings.EqualFold(ru.Host, u.Host) {
			return true
		}
	}
	return OriginAllowed(allowlist, false, candidate) && len(allowlist) > 0
}

type SubmitRequest struct {
	Form        *models.Form
	Fields      []models.MessageField
	Attachments []models.InboundAttachmentMeta
	ClientIP    string
	UserAgent   string
	Referer     string
	Origin      string
	HoneypotHit bool
	NonceFailed bool
}

type SubmitResult struct {
	Message   *models.Message
	Duplicate bool
	Stored    bool
}

func (s *Service) Submit(ctx context.Context, req SubmitRequest) (*SubmitResult, error) {
	form := req.Form
	if form == nil {
		return nil, ErrFormNotFound
	}
	if !form.IsActive() {
		return nil, ErrFormInactive
	}
	if len(req.Fields) > form.FieldLimit() {
		return nil, ErrTooManyFields
	}
	if form.WorkspaceID == nil {
		return nil, ErrFormNotFound
	}

	honeypot := strings.ToLower(form.Honeypot())
	fields := make([]models.MessageField, 0, len(req.Fields))
	for _, f := range req.Fields {
		key := SanitizeHeaderValue(f.Key)
		if key == "" || IsReserved(key) || strings.ToLower(key) == honeypot {
			continue
		}
		fields = append(fields, models.MessageField{Key: key, Value: Sanitize(f.Value)})
	}

	extracted := Extract(fields)
	senderEmail := NormalizeEmail(extracted.Email)

	if err := s.checkQuotas(ctx, form.ID, *form.WorkspaceID, req.ClientIP, senderEmail, req.HoneypotHit); err != nil {
		return nil, err
	}

	hash := dedupHash(form.ID, senderEmail, extracted.Body)
	if existing, err := s.messageRepo.FindRecentDuplicate(form.ID, hash, time.Now().Add(-dedupWindow)); err == nil && existing != nil {
		return &SubmitResult{Message: existing, Duplicate: true, Stored: true}, nil
	}

	verdict := s.scanner.Scan(messagescan.Input{
		Policy:      form,
		WorkspaceID: *form.WorkspaceID,
		SenderEmail: senderEmail,
		SenderName:  extracted.Name,
		Subject:     extracted.Subject,
		Body:        extracted.Body,
		Fields:      fields,
		ClientIP:    req.ClientIP,
		UserAgent:   req.UserAgent,
		HoneypotHit: req.HoneypotHit,
		NonceFailed: req.NonceFailed,
	})

	threadToken := newThreadToken()
	if threadToken == "" {
		return nil, errors.New("failed to generate a thread token")
	}

	subject := extracted.Subject
	if subject == "" {
		subject = defaultSubject(form)
	}

	fieldsJSON, _ := json.Marshal(fields)
	attachmentsJSON := ""
	if len(req.Attachments) > 0 && form.AllowAttachments {
		if raw, err := json.Marshal(req.Attachments); err == nil {
			attachmentsJSON = string(raw)
		}
	}

	msg := &models.Message{
		WorkspaceID:     form.WorkspaceID,
		FormID:          form.ID,
		SenderEmail:     senderEmail,
		SenderName:      extracted.Name,
		SenderPhone:     extracted.Phone,
		Subject:         subject,
		Body:            extracted.Body,
		FieldsJSON:      string(fieldsJSON),
		AttachmentsJSON: attachmentsJSON,
		ClientIP:        req.ClientIP,
		UserAgent:       truncate(req.UserAgent, 400),
		Referer:         truncate(req.Referer, 400),
		Origin:          truncate(req.Origin, 200),
		Status:          verdict.Status(),
		State:           models.MessageStateNew,
		SpamScore:       verdict.Score,
		ScanReasons:     verdict.Reasons,
		ThreadToken:     threadToken,
		DedupHash:       hash,
	}
	if msg.IsSpam() {
		msg.State = models.MessageStateSpam
	}

	if err := s.messageRepo.Create(msg); err != nil {
		return nil, err
	}

	_ = s.formRepo.RecordSubmission(form.ID, msg.IsSpam(), msg.CreatedAt)
	if len(verdict.FilterHits) > 0 {
		_ = s.filterRepo.RecordHits(verdict.FilterHits, time.Now())
	}
	if s.onReceived != nil {
		s.onReceived(msg.Status)
	}

	if msg.Status != models.MessageStatusRejected && s.enqueuer != nil {
		if err := s.enqueuer.EnqueueMessageProcess(msg.ID); err != nil {
			logger.Error("failed to enqueue message processing", "message_id", msg.ID, "error", err)
		}
	}

	return &SubmitResult{Message: msg, Stored: true}, nil
}

type ReplyRequest struct {
	HTML        string
	Text        string
	Subject     string
	Attachments []models.Attachment
}

func (s *Service) Reply(ctx context.Context, actorID, apiKeyID uint, actorEmail string, form *models.Form, msg *models.Message, req ReplyRequest) (*models.MessageReply, error) {
	if s.emailService == nil {
		return nil, errors.New("email service unavailable")
	}
	if !msg.CanReply() {
		return nil, ErrNoRecipient
	}

	from := strings.TrimSpace(form.ReplyFrom)
	if from == "" {
		return nil, ErrSenderNotSet
	}
	if form.ReplyFromName != "" {
		from = fmt.Sprintf("%s <%s>", SanitizeHeaderValue(form.ReplyFromName), from)
	}

	subject := SanitizeHeaderValue(req.Subject)
	if subject == "" {
		subject = replySubject(msg.Subject)
	}

	messageID := newMessageID(msg.UUID, from)
	headers := map[string]string{"Message-ID": messageID}
	if msg.RootMessageID != "" {
		prior, err := s.messageRepo.ReplyMessageIDs(msg.ID)
		if err != nil {
			logger.Warn("failed to load thread message ids", "message_id", msg.ID, "error", err)
		}
		inReplyTo, references := threadHeaders(msg.RootMessageID, prior)
		headers["In-Reply-To"] = inReplyTo
		headers["References"] = references
	}
	if replyTo := s.threadReplyTo(msg); replyTo != "" {
		headers["Reply-To"] = replyTo
	}

	sendReq := &email.SendRequest{
		From:          from,
		To:            []string{msg.SenderEmail},
		Subject:       subject,
		HTML:          req.HTML,
		Text:          req.Text,
		Attachments:   req.Attachments,
		SystemHeaders: headers,
	}

	resp, err := s.emailService.Send(ctx, actorID, apiKeyID, msg.WorkspaceID, actorEmail, sendReq)
	if err != nil {
		return nil, err
	}

	var authorID *uint
	if actorID != 0 {
		authorID = &actorID
	}
	reply := &models.MessageReply{
		MessageID:       msg.ID,
		WorkspaceID:     msg.WorkspaceID,
		Kind:            models.MessageReplyKindOperator,
		AuthorID:        authorID,
		FromAddr:        from,
		ToAddr:          msg.SenderEmail,
		Subject:         subject,
		HTMLBody:        req.HTML,
		TextBody:        req.Text,
		EmailUUID:       resp.ID,
		MessageIDHeader: messageID,
	}
	if err := s.messageRepo.AddReply(reply); err != nil {
		return nil, err
	}
	if s.enqueuer != nil {
		if err := s.enqueuer.EnqueueMessageReply(reply.ID); err != nil {
			logger.Error("failed to enqueue message reply webhook", "reply_id", reply.ID, "error", err)
		}
	}

	if msg.RootMessageID == "" {
		if err := s.messageRepo.SetRootMessageID(msg.ID, messageID); err != nil {
			logger.Error("failed to record thread root message id", "message_id", msg.ID, "error", err)
		} else {
			msg.RootMessageID = messageID
		}
	}

	now := time.Now()
	if err := s.messageRepo.RecordReply(msg.ID, now); err != nil {
		logger.Error("failed to update message after reply", "message_id", msg.ID, "error", err)
	}
	msg.RepliedAt = &now
	msg.UpdatedAt = &now
	msg.ReplyCount++
	if msg.State != models.MessageStateClosed {
		msg.State = models.MessageStateReplied
	}
	if msg.ReadAt == nil {
		msg.ReadAt = &now
	}

	return reply, nil
}

func (s *Service) threadReplyTo(msg *models.Message) string {
	if s.cfg.InboundDomain == "" || msg.ThreadToken == "" {
		return ""
	}
	return fmt.Sprintf("msg+%s@%s", msg.ThreadToken, s.cfg.InboundDomain)
}

func (s *Service) ThreadReplyTo(msg *models.Message) string { return s.threadReplyTo(msg) }

// maxReferences caps the References header at the thread root plus the most
// recent replies; RFC 5322 §3.6.4 lets a long thread trim the middle.
const maxReferences = 10

// threadHeaders returns the In-Reply-To and References values for a new reply
// on a thread whose first reply was root and whose later replies are prior,
// oldest first.
func threadHeaders(root string, prior []string) (inReplyTo, references string) {
	chain := make([]string, 0, maxReferences+1)
	chain = append(chain, root)
	recent := make([]string, 0, len(prior))
	for _, id := range prior {
		if id != "" && id != root {
			recent = append(recent, id)
		}
	}
	if len(recent) > maxReferences {
		recent = recent[len(recent)-maxReferences:]
	}
	chain = append(chain, recent...)
	return chain[len(chain)-1], strings.Join(chain, " ")
}

// newMessageID builds an RFC 5322 Message-ID on the sending domain, so it is
// covered by the same DKIM signature as the rest of the message.
func newMessageID(messageUUID, from string) string {
	domain := "posta.local"
	if addr, err := mail.ParseAddress(from); err == nil {
		if at := strings.LastIndex(addr.Address, "@"); at >= 0 && at+1 < len(addr.Address) {
			domain = strings.ToLower(addr.Address[at+1:])
		}
	}
	return fmt.Sprintf("<%s.%s@%s>", uuid.NewString(), messageUUID, domain)
}

func defaultSubject(form *models.Form) string {
	return fmt.Sprintf("New submission on %s", form.Name)
}

func replySubject(subject string) string {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return "Re: your message"
	}
	if strings.HasPrefix(strings.ToLower(subject), "re:") {
		return subject
	}
	return "Re: " + subject
}

func dedupHash(formID uint, email, body string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d|%s|%s", formID, strings.ToLower(email), body)))
	return hex.EncodeToString(sum[:])
}

func newThreadToken() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return ""
	}
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw))
}

func truncate(v string, n int) string {
	if len(v) <= n {
		return v
	}
	return v[:n]
}

type lookup struct {
	messageRepo     *repositories.MessageRepository
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
	return l.messageRepo.CountByIPSince(ip, since)
}

func (l *lookup) SubmissionsFromSender(src models.ScanSource, email string, since time.Time) int64 {
	if src.Kind != models.ScanSourceForm {
		return 0
	}
	return l.messageRepo.CountBySenderSince(src.ID, email, since)
}

// ActiveFilters returns the workspace-wide filters plus those scoped to the
// source. Only forms carry source-scoped filters today.
func (l *lookup) ActiveFilters(workspaceID uint, src models.ScanSource) ([]models.MessageFilter, error) {
	var formID uint
	if src.Kind == models.ScanSourceForm {
		formID = src.ID
	}
	return l.filterRepo.FindActive(workspaceID, formID)
}
