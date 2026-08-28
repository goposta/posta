// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/audit"
	"github.com/goposta/posta/internal/services/eventbus"
	"github.com/goposta/posta/internal/services/messages"
	"github.com/goposta/posta/internal/storage/blob"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/logger"
	"github.com/jkaninda/okapi"
)

const (
	metaMessageID = "message_id"
	metaFormID    = "form_id"
	metaEmailUUID = "email_uuid"
)

type MessageHandler struct {
	repo       *repositories.MessageRepository
	formRepo   *repositories.FormRepository
	filterRepo *repositories.MessageFilterRepository
	svc        *messages.Service
	blobStore  blob.Store
	bus        *eventbus.EventBus
	audit      *audit.Logger
}

func NewMessageHandler(
	repo *repositories.MessageRepository,
	formRepo *repositories.FormRepository,
	filterRepo *repositories.MessageFilterRepository,
	svc *messages.Service,
	auditLogger *audit.Logger,
) *MessageHandler {
	return &MessageHandler{repo: repo, formRepo: formRepo, filterRepo: filterRepo, svc: svc, audit: auditLogger}
}

func (h *MessageHandler) SetBlobStore(bs blob.Store)       { h.blobStore = bs }
func (h *MessageHandler) SetEventBus(b *eventbus.EventBus) { h.bus = b }

type MessageListRequest struct {
	Page     int    `query:"page" default:"0"`
	Size     int    `query:"size" default:"20"`
	FormID   int    `query:"form_id"`
	Status   string `query:"status" enum:"received,flagged,quarantined,rejected"`
	State    string `query:"state" enum:"new,open,replied,closed,spam"`
	Unread   bool   `query:"unread"`
	Q        string `query:"q"`
	After    string `query:"after" doc:"RFC 3339 lower bound on received time"`
	Before   string `query:"before" doc:"RFC 3339 upper bound on received time"`
	Assigned int    `query:"assigned_to"`
}

type MessageIDRequest struct {
	ID string `param:"id"`
}

type MessageAttachmentRequest struct {
	ID  string `param:"id"`
	Idx int    `param:"idx"`
}

type ReplyMessageRequest struct {
	ID   string `param:"id"`
	Body struct {
		Subject string `json:"subject"`
		HTML    string `json:"html"`
		Text    string `json:"text"`
	} `json:"body"`
}

type UpdateMessageStateRequest struct {
	ID   string `param:"id"`
	Body struct {
		State string `json:"state" required:"true" enum:"new,open,replied,closed,spam"`
		Read  *bool  `json:"read"`
	} `json:"body"`
}

type AssignMessageRequest struct {
	ID   string `param:"id"`
	Body struct {
		UserID *uint `json:"user_id"`
	} `json:"body"`
}

type MarkSpamRequest struct {
	ID   string `param:"id"`
	Body struct {
		CreateFilter bool   `json:"create_filter"`
		Pattern      string `json:"pattern"`
		Kind         string `json:"kind" enum:"keyword,phrase,email,domain,ip"`
	} `json:"body"`
}

type MessageStatsResponse struct {
	Total  int64 `json:"total"`
	Unread int64 `json:"unread"`
	Spam   int64 `json:"spam"`
	Forms  int64 `json:"forms"`
}

type MessageAnalyticsRequest struct {
	Days int `query:"days" default:"30"`
}

type MessageAnalyticsResponse struct {
	Daily []repositories.MessageDailyCount `json:"daily"`
	Total int64                            `json:"total"`
	Spam  int64                            `json:"spam"`
}

func (h *MessageHandler) List(c *okapi.Context, req *MessageListRequest) error {
	page, size, offset := normalizePageParams(req.Page, req.Size)

	filter := repositories.MessageFilterQuery{
		FormID: uint(req.FormID),
		Status: req.Status,
		State:  req.State,
		Unread: req.Unread,
		Query:  strings.TrimSpace(req.Q),
	}
	if req.Assigned > 0 {
		assigned := uint(req.Assigned)
		filter.Assigned = &assigned
	}
	if t, err := time.Parse(time.RFC3339, req.After); err == nil {
		filter.From = &t
	}
	if t, err := time.Parse(time.RFC3339, req.Before); err == nil {
		filter.To = &t
	}

	items, total, err := h.repo.FindByScopeFiltered(getScope(c), filter, size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list messages")
	}
	for i := range items {
		hydrate(&items[i])
		items[i].Body = truncateBody(items[i].Body, 400)
	}
	return paginated(c, items, total, page, size)
}

func (h *MessageHandler) Get(c *okapi.Context, req *MessageIDRequest) error {
	msg, err := h.repo.FindByUUIDForScope(getScope(c), req.ID)
	if err != nil {
		return c.AbortNotFound("message not found")
	}
	hydrate(msg)

	if msg.ReadAt == nil {
		now := time.Now()
		msg.ReadAt = &now
		if msg.State == models.MessageStateNew {
			msg.State = models.MessageStateOpen
		}
		if err := h.repo.Update(msg); err != nil {
			return c.AbortInternalServerError("failed to update message")
		}
	}
	return ok(c, msg)
}

func (h *MessageHandler) Delete(c *okapi.Context, req *MessageIDRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	msg, err := h.repo.FindByUUIDForScope(getScope(c), req.ID)
	if err != nil {
		return c.AbortNotFound("message not found")
	}
	if err := h.repo.Delete(msg.ID); err != nil {
		return c.AbortInternalServerError("failed to delete message")
	}
	if h.audit != nil {
		h.audit.LogCtx(c, "message.deleted", fmt.Sprintf("Message %s deleted", msg.UUID), map[string]any{metaMessageID: msg.UUID})
	}
	return noContent(c)
}

func (h *MessageHandler) Reply(c *okapi.Context, req *ReplyMessageRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	if strings.TrimSpace(req.Body.HTML) == "" && strings.TrimSpace(req.Body.Text) == "" {
		return c.AbortBadRequest("a reply must include html or text content")
	}

	scope := getScope(c)
	msg, err := h.repo.FindByUUIDForScope(scope, req.ID)
	if err != nil {
		return c.AbortNotFound("message not found")
	}
	form, err := h.formRepo.FindByIDForScope(scope, msg.FormID)
	if err != nil {
		return c.AbortNotFound("form not found")
	}

	reply, err := h.svc.Reply(
		c.Request().Context(),
		uint(c.GetInt("user_id")),
		uint(c.GetInt("api_key_id")),
		c.GetString("user_email"),
		form, msg,
		messages.ReplyRequest{
			Subject: req.Body.Subject,
			HTML:    req.Body.HTML,
			Text:    req.Body.Text,
		},
	)
	if err != nil {
		switch {
		case err == messages.ErrSenderNotSet:
			return c.AbortBadRequest("configure a reply sender address on this form before replying")
		case err == messages.ErrNoRecipient:
			return c.AbortBadRequest("this message has no reply address")
		case isRateLimitError(err):
			return c.AbortTooManyRequests(err.Error())
		case isDomainVerificationError(err):
			return c.AbortForbidden("sender domain is not verified", err)
		}
		return c.AbortInternalServerError(err.Error())
	}

	if h.audit != nil {
		h.audit.LogCtx(c, "message.replied", fmt.Sprintf("Replied to message %s", msg.UUID), map[string]any{
			metaMessageID: msg.UUID, metaEmailUUID: reply.EmailUUID,
		})
	}
	h.publish(c, "message.replied", msg)
	return created(c, reply)
}

func (h *MessageHandler) UpdateState(c *okapi.Context, req *UpdateMessageStateRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	msg, err := h.repo.FindByUUIDForScope(getScope(c), req.ID)
	if err != nil {
		return c.AbortNotFound("message not found")
	}

	state := models.MessageState(req.Body.State)
	switch state {
	case models.MessageStateNew, models.MessageStateOpen, models.MessageStateReplied,
		models.MessageStateClosed, models.MessageStateSpam:
	default:
		return c.AbortBadRequest("invalid state")
	}

	now := time.Now()
	msg.State = state
	msg.UpdatedAt = &now
	if req.Body.Read != nil {
		if *req.Body.Read {
			msg.ReadAt = &now
		} else {
			msg.ReadAt = nil
		}
	}
	if err := h.repo.Update(msg); err != nil {
		return c.AbortInternalServerError("failed to update message")
	}
	hydrate(msg)
	return ok(c, msg)
}

func (h *MessageHandler) Assign(c *okapi.Context, req *AssignMessageRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	msg, err := h.repo.FindByUUIDForScope(getScope(c), req.ID)
	if err != nil {
		return c.AbortNotFound("message not found")
	}
	now := time.Now()
	msg.AssignedToID = req.Body.UserID
	msg.UpdatedAt = &now
	if err := h.repo.Update(msg); err != nil {
		return c.AbortInternalServerError("failed to assign message")
	}
	hydrate(msg)
	return ok(c, msg)
}

func (h *MessageHandler) MarkSpam(c *okapi.Context, req *MarkSpamRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	scope := getScope(c)
	msg, err := h.repo.FindByUUIDForScope(scope, req.ID)
	if err != nil {
		return c.AbortNotFound("message not found")
	}

	now := time.Now()
	msg.Status = models.MessageStatusQuarantined
	msg.State = models.MessageStateSpam
	msg.UpdatedAt = &now
	if err := h.repo.Update(msg); err != nil {
		return c.AbortInternalServerError("failed to update message")
	}

	if req.Body.CreateFilter {
		kind := models.FilterKind(req.Body.Kind)
		if !models.ValidFilterKinds[kind] {
			kind = models.FilterKindKeyword
		}
		pattern := strings.TrimSpace(req.Body.Pattern)
		if pattern == "" && kind == models.FilterKindDomain {
			pattern = senderDomain(msg.SenderEmail)
		}
		if pattern == "" && kind == models.FilterKindEmail {
			pattern = msg.SenderEmail
		}
		if pattern != "" {
			filter := &models.MessageFilter{
				WorkspaceID: scope.WorkspaceID,
				Kind:        kind,
				Pattern:     pattern,
				Action:      models.FilterActionQuarantine,
				Score:       6,
				Enabled:     true,
				Note:        fmt.Sprintf("created from message %s", msg.UUID),
			}
			if err := h.filterRepo.Create(filter); err != nil {
				return c.AbortInternalServerError("message marked as spam but the filter could not be created")
			}
		}
	}

	hydrate(msg)
	return ok(c, msg)
}

func (h *MessageHandler) MarkNotSpam(c *okapi.Context, req *MessageIDRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	msg, err := h.repo.FindByUUIDForScope(getScope(c), req.ID)
	if err != nil {
		return c.AbortNotFound("message not found")
	}
	now := time.Now()
	msg.Status = models.MessageStatusReceived
	msg.SpamScore = 0
	msg.ScanReasons = nil
	msg.UpdatedAt = &now
	if msg.State == models.MessageStateSpam {
		msg.State = models.MessageStateOpen
	}
	if err := h.repo.Update(msg); err != nil {
		return c.AbortInternalServerError("failed to update message")
	}
	hydrate(msg)
	return ok(c, msg)
}

func (h *MessageHandler) Stats(c *okapi.Context) error {
	scope := getScope(c)
	total, unread, spam := h.repo.CountByScope(scope)
	forms, _ := h.formRepo.CountByScope(scope)
	return ok(c, MessageStatsResponse{Total: total, Unread: unread, Spam: spam, Forms: forms})
}

func (h *MessageHandler) Analytics(c *okapi.Context, req *MessageAnalyticsRequest) error {
	days := req.Days
	if days <= 0 || days > 365 {
		days = 30
	}
	scope := getScope(c)
	rows, err := h.repo.DailyCounts(scope, time.Now().AddDate(0, 0, -days))
	if err != nil {
		return c.AbortInternalServerError("failed to load message analytics")
	}
	var total, spam int64
	for _, r := range rows {
		total += r.Total
		spam += r.Spam
	}
	return ok(c, MessageAnalyticsResponse{Daily: rows, Total: total, Spam: spam})
}

func (h *MessageHandler) DownloadAttachment(c *okapi.Context, req *MessageAttachmentRequest) error {
	msg, err := h.repo.FindByUUIDForScope(getScope(c), req.ID)
	if err != nil {
		return c.AbortNotFound("message not found")
	}
	hydrate(msg)
	if req.Idx < 0 || req.Idx >= len(msg.Attachments) {
		return c.AbortNotFound("attachment not found")
	}
	att := msg.Attachments[req.Idx]
	if h.blobStore == nil || att.StorageKey == "" {
		return c.AbortNotFound("attachment content is unavailable")
	}
	rc, err := h.blobStore.Get(c.Request().Context(), att.StorageKey)
	if err != nil {
		return c.AbortNotFound("attachment content is unavailable")
	}
	defer func() { _ = rc.Close() }()

	contentType := att.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.ResponseWriter().Header().Set("Content-Type", contentType)
	c.ResponseWriter().Header().Set("Content-Disposition", contentDisposition(att.Filename))
	c.ResponseWriter().Header().Set("X-Content-Type-Options", "nosniff")
	c.ResponseWriter().WriteHeader(http.StatusOK)
	if _, err := io.Copy(c.ResponseWriter(), rc); err != nil {
		logger.Warn("message attachment stream failed", "uuid", msg.UUID, "error", err)
	}
	return nil
}

func (h *MessageHandler) Stream(c *okapi.Context) error {
	if h.bus == nil {
		return c.AbortNotFound("message stream not configured")
	}
	ctx := c.Request().Context()
	scope := getScope(c)
	if scope.WorkspaceID == nil {
		return c.AbortBadRequest("a workspace is required")
	}
	workspaceID := *scope.WorkspaceID

	ch, unsub := h.bus.Subscribe()
	defer unsub()

	msgCh := make(chan okapi.Message, 4)
	msgCh <- okapi.Message{
		Event: "system.info",
		Data: okapi.M{
			"workspace_id": workspaceID,
			"timestamp":    time.Now().UTC().Format(time.RFC3339),
		},
	}

	go func() {
		defer close(msgCh)
		for {
			select {
			case <-ctx.Done():
				return
			case evt, okEvt := <-ch:
				if !okEvt {
					return
				}
				if !strings.HasPrefix(evt.Type, "message.") {
					continue
				}
				if evt.WorkspaceID == nil || *evt.WorkspaceID != workspaceID {
					continue
				}
				select {
				case msgCh <- okapi.Message{Event: evt.Type, Data: evt}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return c.SSEStreamWithOptions(ctx, msgCh, &okapi.StreamOptions{
		Serializer:   &okapi.JSONSerializer{},
		PingInterval: 30 * time.Second,
	})
}

func (h *MessageHandler) publish(c *okapi.Context, eventType string, msg *models.Message) {
	if h.bus == nil {
		return
	}
	actor := uint(c.GetInt("user_id"))
	h.bus.PublishScoped(msg.WorkspaceID, models.EventCategoryEmail, eventType, &actor,
		c.GetString("user_email"), c.RealIP(),
		fmt.Sprintf("Message %s %s", msg.UUID, strings.TrimPrefix(eventType, "message.")),
		map[string]any{metaMessageID: msg.UUID, metaFormID: msg.FormID})
}

func hydrate(msg *models.Message) {
	if msg.FieldsJSON != "" {
		var fields []models.MessageField
		if err := json.Unmarshal([]byte(msg.FieldsJSON), &fields); err == nil {
			msg.Fields = fields
		}
	}
	if msg.AttachmentsJSON != "" {
		var atts []models.InboundAttachmentMeta
		if err := json.Unmarshal([]byte(msg.AttachmentsJSON), &atts); err == nil {
			for i := range atts {
				atts[i].Content = ""
			}
			msg.Attachments = atts
		}
	}
}

func truncateBody(body string, n int) string {
	runes := []rune(body)
	if len(runes) <= n {
		return body
	}
	return string(runes[:n]) + "…"
}

func senderDomain(email string) string {
	at := strings.LastIndex(email, "@")
	if at < 0 || at == len(email)-1 {
		return ""
	}
	return strings.ToLower(email[at+1:])
}
