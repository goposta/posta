// SPDX-FileCopyrightText: 2026 Jonas Kaninda
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
	"net/url"
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/email"
	"github.com/goposta/posta/internal/services/eventbus"
	"github.com/goposta/posta/internal/services/messagescan"
	"github.com/goposta/posta/internal/storage/blob"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/logger"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
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
}

type Service struct {
	formRepo        *repositories.FormRepository
	messageRepo     *repositories.MessageRepository
	filterRepo      *repositories.MessageFilterRepository
	suppressionRepo *repositories.SuppressionRepository
	emailService    *email.Service
	scanner         *messagescan.Scanner
	redis           *redis.Client
	blobStore       blob.Store
	bus             *eventbus.EventBus
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
	s.scanner = messagescan.New(&lookup{
		messageRepo:     messageRepo,
		filterRepo:      filterRepo,
		suppressionRepo: suppressionRepo,
	})
	return s
}

func (s *Service) SetEmailService(es *email.Service)        { s.emailService = es }
func (s *Service) SetBlobStore(bs blob.Store)               { s.blobStore = bs }
func (s *Service) SetEventBus(b *eventbus.EventBus)         { s.bus = b }
func (s *Service) SetEnqueuer(e Enqueuer)                   { s.enqueuer = e }
func (s *Service) OnReceived(fn func(models.MessageStatus)) { s.onReceived = fn }

func (s *Service) Scanner() *messagescan.Scanner { return s.scanner }

func (s *Service) FindFormByKey(key string) (*models.Form, error) {
	f, err := s.formRepo.FindByPublicKey(key)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFormNotFound
		}
		return nil, err
	}
	return f, nil
}

func OriginAllowed(form *models.Form, origin string) bool {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return !form.StrictOrigin
	}
	if len(form.AllowedOrigins) == 0 {
		return true
	}
	for _, allowed := range form.AllowedOrigins {
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

func RedirectAllowed(form *models.Form, target string) bool {
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
	if form.RedirectURL != "" {
		if ru, err := url.Parse(form.RedirectURL); err == nil && strings.EqualFold(ru.Host, u.Host) {
			return true
		}
	}
	return OriginAllowed(form, candidate) && len(form.AllowedOrigins) > 0
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

	if err := s.checkQuotas(ctx, form.ID, *form.WorkspaceID, req.ClientIP, senderEmail); err != nil {
		return nil, err
	}

	hash := dedupHash(form.ID, senderEmail, extracted.Body)
	if existing, err := s.messageRepo.FindRecentDuplicate(form.ID, hash, time.Now().Add(-dedupWindow)); err == nil && existing != nil {
		return &SubmitResult{Message: existing, Duplicate: true, Stored: true}, nil
	}

	verdict := s.scanner.Scan(messagescan.Input{
		Form:        form,
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

	headers := map[string]string{}
	if msg.RootMessageID != "" {
		headers["In-Reply-To"] = msg.RootMessageID
		headers["References"] = msg.RootMessageID
	}
	if replyTo := s.threadReplyTo(msg); replyTo != "" {
		headers["Reply-To"] = replyTo
	}

	sendReq := &email.SendRequest{
		From:        from,
		To:          []string{msg.SenderEmail},
		Subject:     subject,
		HTML:        req.HTML,
		Text:        req.Text,
		Attachments: req.Attachments,
		Headers:     headers,
	}

	resp, err := s.emailService.Send(ctx, actorID, apiKeyID, msg.WorkspaceID, actorEmail, sendReq)
	if err != nil {
		return nil, err
	}

	reply := &models.MessageReply{
		MessageID:   msg.ID,
		WorkspaceID: msg.WorkspaceID,
		Kind:        models.MessageReplyKindOperator,
		AuthorID:    actorID,
		FromAddr:    from,
		ToAddr:      msg.SenderEmail,
		Subject:     subject,
		HTMLBody:    req.HTML,
		TextBody:    req.Text,
		EmailUUID:   resp.ID,
	}
	if err := s.messageRepo.AddReply(reply); err != nil {
		return nil, err
	}

	now := time.Now()
	msg.RepliedAt = &now
	msg.UpdatedAt = &now
	msg.ReplyCount++
	if msg.State != models.MessageStateClosed {
		msg.State = models.MessageStateReplied
	}
	if msg.ReadAt == nil {
		msg.ReadAt = &now
	}
	if err := s.messageRepo.Update(msg); err != nil {
		logger.Error("failed to update message after reply", "message_id", msg.ID, "error", err)
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

func (l *lookup) SubmissionsFromSender(formID uint, email string, since time.Time) int64 {
	return l.messageRepo.CountBySenderSince(formID, email, since)
}

func (l *lookup) ActiveFilters(workspaceID uint, formID uint) ([]models.MessageFilter, error) {
	return l.filterRepo.FindActive(workspaceID, formID)
}
