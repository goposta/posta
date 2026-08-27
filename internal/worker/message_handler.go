// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/notification"
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
	Subject     string                `json:"subject,omitempty"`
	Body        string                `json:"body,omitempty"`
	Fields      []models.MessageField `json:"fields,omitempty"`
	Status      string                `json:"status"`
	SpamScore   float64               `json:"spam_score"`
	ScanReasons []string              `json:"scan_reasons,omitempty"`
	ClientIP    string                `json:"client_ip,omitempty"`
	ReceivedAt  string                `json:"received_at"`
}

type MessageProcessHandler struct {
	messageRepo   *repositories.MessageRepository
	formRepo      *repositories.FormRepository
	workspaceRepo *repositories.WorkspaceRepository
	dispatcher    *webhook.Dispatcher
	notifier      *notification.Service
	appURL        string
	onNotified    func()
}

func NewMessageProcessHandler(
	messageRepo *repositories.MessageRepository,
	formRepo *repositories.FormRepository,
	workspaceRepo *repositories.WorkspaceRepository,
	dispatcher *webhook.Dispatcher,
	notifier *notification.Service,
	appURL string,
) *MessageProcessHandler {
	return &MessageProcessHandler{
		messageRepo:   messageRepo,
		formRepo:      formRepo,
		workspaceRepo: workspaceRepo,
		dispatcher:    dispatcher,
		notifier:      notifier,
		appURL:        strings.TrimSuffix(appURL, "/"),
	}
}

func (h *MessageProcessHandler) OnNotified(fn func()) { h.onNotified = fn }

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

	h.dispatch(msg, form)

	if msg.NotifiedAt == nil && h.shouldNotify(msg, form) {
		if err := h.notify(msg, form); err != nil {
			logger.Error("failed to send message notification", "message_id", msg.ID, "error", err)
		} else {
			now := time.Now().UTC()
			msg.NotifiedAt = &now
			if err := h.messageRepo.Update(msg); err != nil {
				logger.Error("failed to mark message notified", "message_id", msg.ID, "error", err)
			}
			if h.onNotified != nil {
				h.onNotified()
			}
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

	workspaceName := ""
	if msg.WorkspaceID != nil && h.workspaceRepo != nil {
		if ws, err := h.workspaceRepo.FindByID(*msg.WorkspaceID); err == nil {
			workspaceName = ws.Name
		}
	}

	subject := fmt.Sprintf("New message on %s", form.Name)
	if msg.Status == models.MessageStatusFlagged {
		subject = fmt.Sprintf("Flagged message on %s", form.Name)
	}

	data := map[string]any{
		"FormName":   form.Name,
		"Fields":     notifiableFields(msg, fields),
		"Body":       msg.Body,
		"Flagged":    msg.Status == models.MessageStatusFlagged,
		"SpamScore":  fmt.Sprintf("%.1f", msg.SpamScore),
		"MessageURL": h.messageURL(msg.UUID),
	}
	if workspaceName != "" {
		data["WorkspaceName"] = workspaceName
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

func notifiableFields(msg *models.Message, fields []models.MessageField) []models.MessageField {
	out := []models.MessageField{
		{Key: "From", Value: displaySender(msg)},
		{Key: "Subject", Value: msg.Subject},
	}
	for _, f := range fields {
		if strings.EqualFold(f.Key, "message") || strings.EqualFold(f.Key, "body") {
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
