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

package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/goposta/posta/internal/dto"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/messages"
	"github.com/goposta/posta/internal/services/notification"
	"github.com/goposta/posta/internal/services/realtime"
	"github.com/goposta/posta/internal/services/webhook"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/hibiken/asynq"
	"github.com/jkaninda/logger"
)

type MessageWebhookPayload struct {
	Event       string                `json:"event"`
	Timestamp   string                `json:"timestamp"`
	MessageID   string                `json:"message_id"`
	FormID      string                `json:"form_id"`
	FormName    string                `json:"form_name"`
	SenderEmail string                `json:"sender_email,omitempty"`
	SenderName  string                `json:"sender_name,omitempty"`
	SenderPhone string                `json:"sender_phone,omitempty"`
	Subject     string                `json:"subject,omitempty"`
	Body        string                `json:"body,omitempty"`
	Fields      []models.MessageField `json:"fields,omitempty"`
	Status      string                `json:"status"`
	SpamScore   float64               `json:"spam_score"`
	ScanReasons []string              `json:"scan_reasons,omitempty"`
	ClientIP    string                `json:"client_ip,omitempty"`
	ReceivedAt  string                `json:"received_at"`
}

// messageStore is the slice of MessageRepository the worker needs; an interface
// so the retry semantics can be tested without a database.
type messageStore interface {
	FindByID(id uint) (*models.Message, error)
	UpdateFields(id uint, fields map[string]any) error
	MarkNotified(ids []uint, at time.Time) error
	FindReplyByID(id uint) (*models.MessageReply, error)
}

type formStore interface {
	FindByID(id uint) (*models.Form, error)
}

type jsonDispatcher interface {
	DispatchJSON(userID uint, workspaceID *uint, event string, body []byte, fromAddr string)
}

type messageNotifier interface {
	IsConfigured() bool
	Send(to, subject, templateName string, data map[string]any) error
	SendToUser(userID uint, subject, templateName string, data map[string]any) error
	SendToWorkspaceAdmins(workspaceID uint, subject, templateName string, data map[string]any) error
}

type MessageProcessHandler struct {
	messageRepo   messageStore
	formRepo      formStore
	workspaceRepo *repositories.WorkspaceRepository
	dispatcher    jsonDispatcher
	notifier      messageNotifier
	realtime      realtime.Bus
	appURL        string
	onNotified    func()
	// retryCount reports how many times Asynq has already tried this task.
	retryCount func(ctx context.Context) int
}

func asynqRetryCount(ctx context.Context) int {
	n, _ := asynq.GetRetryCount(ctx)
	return n
}

func NewMessageProcessHandler(
	messageRepo *repositories.MessageRepository,
	formRepo *repositories.FormRepository,
	workspaceRepo *repositories.WorkspaceRepository,
	dispatcher *webhook.Dispatcher,
	notifier *notification.Service,
	appURL string,
) *MessageProcessHandler {
	h := &MessageProcessHandler{
		messageRepo:   messageRepo,
		formRepo:      formRepo,
		workspaceRepo: workspaceRepo,
		appURL:        strings.TrimSuffix(appURL, "/"),
		retryCount:    asynqRetryCount,
	}
	// Assign through nil checks so a nil pointer never becomes a non-nil interface.
	if dispatcher != nil {
		h.dispatcher = dispatcher
	}
	if notifier != nil {
		h.notifier = notifier
	}
	return h
}

func (h *MessageProcessHandler) OnNotified(fn func()) { h.onNotified = fn }

// SetRealtime enables live dashboard events for processed messages.
func (h *MessageProcessHandler) SetRealtime(b realtime.Bus) { h.realtime = b }

// ProcessTask fans a stored submission out to webhooks, the live dashboard, and
// notification email. It is safe to retry: the webhook hand-off and the
// notification are each recorded once done and skipped on a later attempt, and
// a failed notification returns an error so Asynq retries it.
func (h *MessageProcessHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload MessageProcessPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal message payload: %w: %w", err, asynq.SkipRetry)
	}

	msg, err := h.messageRepo.FindByID(payload.MessageID)
	if err != nil {
		return fmt.Errorf("message not found: %w", err)
	}
	if msg.Status == models.MessageStatusRejected {
		return nil
	}

	form, err := h.formRepo.FindByID(msg.FormID)
	if err != nil {
		return fmt.Errorf("form not found: %w", err)
	}

	if msg.WebhookDispatchedAt == nil {
		h.dispatch(msg, form)
		now := time.Now().UTC()
		if err := h.messageRepo.UpdateFields(msg.ID, map[string]any{"webhook_dispatched_at": now}); err != nil {
			logger.Error("failed to mark message webhook dispatched", "message_id", msg.ID, "error", err)
		}
		if !msg.IsSpam() {
			messages.PublishEvent(ctx, h.realtime, messages.EventReceived, msg)
		}
	}

	if msg.NotifiedAt == nil && h.shouldNotify(msg, form) {
		if err := h.notify(msg, form); err != nil {
			return fmt.Errorf("notify message %d: %w", msg.ID, err)
		}
		if err := h.messageRepo.MarkNotified([]uint{msg.ID}, time.Now().UTC()); err != nil {
			logger.Error("failed to mark message notified", "message_id", msg.ID, "error", err)
		}
		if h.onNotified != nil {
			h.onNotified()
		}
	}

	return nil
}

func (h *MessageProcessHandler) shouldNotify(msg *models.Message, form *models.Form) bool {
	if !form.NotifyEnabled || form.NotifyMode != models.NotifyModeImmediate {
		return false
	}
	if msg.Status == models.MessageStatusQuarantined {
		return false
	}
	if msg.Status == models.MessageStatusFlagged && !form.NotifyOnFlagged {
		return false
	}
	return true
}

func (h *MessageProcessHandler) dispatch(msg *models.Message, form *models.Form) {
	if h.dispatcher == nil {
		return
	}

	event := "message.received"
	if msg.IsSpam() {
		event = "message.spam"
	}

	var fields []models.MessageField
	if msg.FieldsJSON != "" {
		_ = json.Unmarshal([]byte(msg.FieldsJSON), &fields)
	}

	body := MessageWebhookPayload{
		Event:       event,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		MessageID:   msg.UUID,
		FormID:      form.UUID,
		FormName:    form.Name,
		SenderEmail: msg.SenderEmail,
		SenderName:  msg.SenderName,
		SenderPhone: msg.SenderPhone,
		Subject:     msg.Subject,
		Body:        msg.Body,
		Fields:      fields,
		Status:      string(msg.Status),
		SpamScore:   msg.SpamScore,
		ScanReasons: []string(msg.ScanReasons),
		ClientIP:    msg.ClientIP,
		ReceivedAt:  msg.CreatedAt.UTC().Format(time.RFC3339),
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		logger.Error("failed to marshal message webhook body", "message_id", msg.ID, "error", err)
		return
	}
	h.dispatcher.DispatchJSON(0, msg.WorkspaceID, event, encoded, msg.SenderEmail)
}

func (h *MessageProcessHandler) notify(msg *models.Message, form *models.Form) error {
	if h.notifier == nil || !h.notifier.IsConfigured() {
		return nil
	}

	var fields []models.MessageField
	if msg.FieldsJSON != "" {
		_ = json.Unmarshal([]byte(msg.FieldsJSON), &fields)
	}

	subject := fmt.Sprintf("New message on %s", form.Name)
	if msg.Status == models.MessageStatusFlagged {
		subject = fmt.Sprintf("Flagged message on %s", form.Name)
	}

	data := map[string]any{
		"FormName":    form.Name,
		"SenderPhone": msg.SenderPhone,
		"Fields":      notifiableFields(msg, fields),
		"Body":        msg.Body,
		"Flagged":     msg.Status == models.MessageStatusFlagged,
		"SpamScore":   fmt.Sprintf("%.1f", msg.SpamScore),
		"MessageURL":  h.messageURL(msg.UUID),
	}

	return h.sendToFormRecipients(msg, form, subject, data)
}

// sendToFormRecipients delivers to the form's explicit notification list, or
// to the workspace owners and admins when the list is empty. With several
// addresses it returns the last error: a retry may then repeat a notification
// to an address that already got it, which beats one never arriving.
func (h *MessageProcessHandler) sendToFormRecipients(msg *models.Message, form *models.Form, subject string, data map[string]any) error {
	if h.workspaceRepo != nil && msg.WorkspaceID != nil {
		if _, ok := data["WorkspaceName"]; !ok {
			if ws, err := h.workspaceRepo.FindByID(*msg.WorkspaceID); err == nil && ws.Name != "" {
				data["WorkspaceName"] = ws.Name
			}
		}
	}
	if len(form.NotifyEmails) > 0 {
		var lastErr error
		for _, to := range form.NotifyEmails {
			to = strings.TrimSpace(to)
			if to == "" {
				continue
			}
			payload := cloneData(data)
			payload["UserName"] = "there"
			if err := h.notifier.Send(to, subject, notification.TemplateNewMessage, payload); err != nil {
				lastErr = err
			}
		}
		return lastErr
	}

	if msg.WorkspaceID == nil {
		return nil
	}
	return h.notifier.SendToWorkspaceAdmins(*msg.WorkspaceID, subject, notification.TemplateNewMessage, data)
}

func (h *MessageProcessHandler) messageURL(uuid string) string {
	if h.appURL == "" {
		return ""
	}
	return fmt.Sprintf("%s/messages/%s", h.appURL, uuid)
}

var summarizedFieldKeys = func() map[string]bool {
	set := map[string]bool{"message": true, "body": true}
	for _, k := range messages.SummarizedKeys() {
		set[strings.ToLower(k)] = true
	}
	return set
}()

func notifiableFields(msg *models.Message, fields []models.MessageField) []models.MessageField {
	out := []models.MessageField{
		{Key: "From", Value: displaySender(msg)},
	}
	if msg.SenderPhone != "" {
		out = append(out, models.MessageField{Key: "Phone", Value: msg.SenderPhone})
	}
	out = append(out, models.MessageField{Key: "Subject", Value: msg.Subject})

	for _, f := range fields {
		if summarizedFieldKeys[strings.ToLower(strings.TrimSpace(f.Key))] {
			continue
		}
		if len(out) >= 12 {
			break
		}
		out = append(out, f)
	}
	return out
}

func displaySender(msg *models.Message) string {
	switch {
	case msg.SenderName != "" && msg.SenderEmail != "":
		return fmt.Sprintf("%s <%s>", msg.SenderName, msg.SenderEmail)
	case msg.SenderEmail != "":
		return msg.SenderEmail
	case msg.SenderName != "":
		return msg.SenderName
	default:
		return "unknown sender"
	}
}

func cloneData(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// ProcessReplyTask sends the follow-up for one thread reply: the
// message.replied webhook for an operator reply, or message.reply_received and
// a notification for the submitter's emailed answer.
//
// The webhook goes out on the first attempt only. Dispatch hands off to the
// dispatcher's own retrying goroutine and cannot fail here, so only the
// notification can make the task retry, and a retry must not re-fire it.
func (h *MessageProcessHandler) ProcessReplyTask(ctx context.Context, t *asynq.Task) error {
	var payload MessageReplyPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal message reply payload: %w: %w", err, asynq.SkipRetry)
	}
	reply, err := h.messageRepo.FindReplyByID(payload.ReplyID)
	if err != nil {
		return fmt.Errorf("message reply not found: %w", err)
	}
	msg, err := h.messageRepo.FindByID(reply.MessageID)
	if err != nil {
		return fmt.Errorf("message not found: %w", err)
	}
	form, err := h.formRepo.FindByID(msg.FormID)
	if err != nil {
		return fmt.Errorf("form not found: %w", err)
	}

	if h.retryCount(ctx) == 0 {
		h.dispatchReply(msg, form, reply)
	}

	if reply.Kind != models.MessageReplyKindInbound || !h.shouldNotifyReply(msg, form) {
		return nil
	}
	if err := h.notifyReply(msg, form, reply); err != nil {
		return fmt.Errorf("notify message reply %d: %w", reply.ID, err)
	}
	return nil
}

func replyEvent(kind models.MessageReplyKind) string {
	if kind == models.MessageReplyKindInbound {
		return messages.EventReplyReceived
	}
	return messages.EventReplied
}

func (h *MessageProcessHandler) dispatchReply(msg *models.Message, form *models.Form, reply *models.MessageReply) {
	if h.dispatcher == nil {
		return
	}
	event := replyEvent(reply.Kind)
	body := dto.MessageReplyWebhookEvent{
		Event:          event,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		MessageID:      msg.UUID,
		ReplyID:        reply.UUID,
		FormID:         form.UUID,
		FormName:       form.Name,
		Kind:           string(reply.Kind),
		From:           reply.FromAddr,
		To:             reply.ToAddr,
		Subject:        reply.Subject,
		TextBody:       reply.TextBody,
		HTMLBody:       reply.HTMLBody,
		EmailID:        reply.EmailUUID,
		SenderMismatch: reply.SenderMismatch,
		CreatedAt:      reply.CreatedAt.UTC().Format(time.RFC3339),
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		logger.Error("failed to marshal message reply webhook body", "reply_id", reply.ID, "error", err)
		return
	}
	h.dispatcher.DispatchJSON(0, msg.WorkspaceID, event, encoded, reply.FromAddr)
}

// shouldNotifyReply follows the form's notification switch, but not its digest
// mode: an answer on a thread someone is working is time-sensitive.
func (h *MessageProcessHandler) shouldNotifyReply(msg *models.Message, form *models.Form) bool {
	return form.NotifyEnabled && form.NotifyMode != models.NotifyModeOff && !msg.IsSpam()
}

// notifyReply tells the assignee, or the form's usual recipients when nobody
// is assigned, that the submitter answered.
func (h *MessageProcessHandler) notifyReply(msg *models.Message, form *models.Form, reply *models.MessageReply) error {
	if h.notifier == nil || !h.notifier.IsConfigured() {
		return nil
	}
	subject := fmt.Sprintf("New reply on %s", form.Name)
	data := map[string]any{
		"FormName":       form.Name,
		"Reply":          true,
		"SenderMismatch": reply.SenderMismatch,
		"Fields": []models.MessageField{
			{Key: "From", Value: reply.FromAddr},
			{Key: "Subject", Value: reply.Subject},
		},
		"Body":       reply.TextBody,
		"MessageURL": h.messageURL(msg.UUID),
	}
	if msg.AssignedToID != nil && *msg.AssignedToID != 0 {
		return h.notifier.SendToUser(*msg.AssignedToID, subject, notification.TemplateNewMessage, data)
	}
	return h.sendToFormRecipients(msg, form, subject, data)
}
