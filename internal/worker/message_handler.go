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
	"github.com/goposta/posta/internal/services/formrules"
	"github.com/goposta/posta/internal/services/messages"
	"github.com/goposta/posta/internal/services/notification"
	"github.com/goposta/posta/internal/services/realtime"
	"github.com/goposta/posta/internal/services/webhook"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/hibiken/asynq"
	"github.com/jkaninda/logger"
	"github.com/lib/pq"
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

	// Typed answers and routing, for forms with questions and rules.
	Data          models.SubmissionData `json:"data,omitempty"`
	SchemaVersion int                   `json:"schema_version,omitempty"`
	Tags          []string              `json:"tags"`
	Priority      string                `json:"priority"`
	TeamID        *uint                 `json:"team_id,omitempty"`
	AssignedToID  *uint                 `json:"assigned_to_id,omitempty"`
	MatchedRules  []string              `json:"matched_rules,omitempty"`
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
	DispatchJSONTo(workspaceID uint, webhookIDs []uint, event string, body []byte)
}

type ruleStore interface {
	ListByForm(formID uint) ([]models.FormRule, error)
	RecordHits(uuids []string, at time.Time) error
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
	rules         ruleStore
	directory     formrules.Directory
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

// SetRouting enables a form's routing rules.
func (h *MessageProcessHandler) SetRouting(rules *repositories.FormRuleRepository, dir formrules.Directory) {
	if rules != nil {
		h.rules = rules
	}
	h.directory = dir
}

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

	outcome := h.route(msg, form)

	if msg.WebhookDispatchedAt == nil {
		h.dispatch(msg, form, outcome.Webhooks)
		now := time.Now().UTC()
		if err := h.messageRepo.UpdateFields(msg.ID, map[string]any{"webhook_dispatched_at": now}); err != nil {
			logger.Error("failed to mark message webhook dispatched", "message_id", msg.ID, "error", err)
		}
		if !msg.IsSpam() {
			messages.PublishEvent(ctx, h.realtime, messages.EventReceived, msg)
		}
	}

	if msg.NotifiedAt == nil && h.shouldNotify(msg, form) {
		if err := h.notify(msg, form, outcome); err != nil {
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

// route applies the form's rules to a submission once: assignment, team,
// tags, priority and the matched rules are stored, and the outcome is
// returned for the webhooks and notifications that follow. On a retry the
// stored matches are replayed instead of re-evaluated, so a rule edited in
// between cannot route the same submission twice.
func (h *MessageProcessHandler) route(msg *models.Message, form *models.Form) formrules.Outcome {
	if h.rules == nil || !formrules.Routable(msg.Status) {
		return formrules.Outcome{}
	}
	rules, err := h.rules.ListByForm(form.ID)
	if err != nil {
		logger.Error("failed to load routing rules", "form_id", form.ID, "error", err)
		return formrules.Outcome{}
	}
	if msg.RoutedAt != nil {
		return formrules.Combine(rules, msg.MatchedRules)
	}
	out := formrules.Evaluate(rules, formrules.FactsFor(msg, form))
	now := time.Now().UTC()
	fields := map[string]any{"routed_at": now}
	if len(out.Matched) > 0 {
		fields["matched_rules"] = pq.StringArray(out.Matched)
		msg.MatchedRules = out.Matched
		if out.AssignUser != nil && msg.AssignedToID == nil && h.isMember(form, *out.AssignUser) {
			fields["assigned_to_id"], msg.AssignedToID = *out.AssignUser, out.AssignUser
		}
		if out.AssignTeam != nil && msg.TeamID == nil && h.ownsTeam(form, *out.AssignTeam) {
			fields["team_id"], msg.TeamID = *out.AssignTeam, out.AssignTeam
		}
		if len(out.Tags) > 0 {
			tags := mergeTags(msg.Tags, out.Tags)
			fields["tags"], msg.Tags = pq.StringArray(tags), tags
		}
		if higherPriority(out.Priority, string(msg.Priority)) {
			fields["priority"], msg.Priority = out.Priority, models.MessagePriority(out.Priority)
		}
	}
	if err := h.messageRepo.UpdateFields(msg.ID, fields); err != nil {
		logger.Error("failed to store routing", "message_id", msg.ID, "error", err)
		return out
	}
	msg.RoutedAt = &now
	if err := h.rules.RecordHits(out.Matched, now); err != nil {
		logger.Warn("failed to count rule hits", "form_id", form.ID, "error", err)
	}
	return out
}

func (h *MessageProcessHandler) isMember(form *models.Form, userID uint) bool {
	if h.directory == nil || form.WorkspaceID == nil {
		return false
	}
	members, err := h.directory.Members(*form.WorkspaceID)
	_, ok := members[userID]
	return err == nil && ok
}

func (h *MessageProcessHandler) ownsTeam(form *models.Form, teamID uint) bool {
	if h.directory == nil || form.WorkspaceID == nil {
		return false
	}
	team, err := h.directory.Team(teamID)
	return err == nil && team.WorkspaceID != nil && *team.WorkspaceID == *form.WorkspaceID
}

// mergeTags adds rule tags to the submission's, keeping within MaxTags.
func mergeTags(current, add []string) []string {
	out := append([]string{}, current...)
	seen := map[string]bool{}
	for _, t := range out {
		seen[t] = true
	}
	for _, t := range add {
		if !seen[t] && len(out) < messages.MaxTags {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

var priorityRank = map[string]int{"low": 1, "normal": 2, "high": 3, "urgent": 4}

// higherPriority: a rule only raises priority, so it never quietens a
// submission an operator already marked urgent.
func higherPriority(candidate, current string) bool {
	if current == "" {
		current = string(models.MessagePriorityNormal)
	}
	return candidate != "" && priorityRank[candidate] > priorityRank[current]
}

func (h *MessageProcessHandler) dispatch(msg *models.Message, form *models.Form, ruleWebhooks []uint) {
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

		Data:          msg.Data,
		SchemaVersion: msg.SchemaVersion,
		Tags:          []string(msg.Tags),
		Priority:      string(msg.Priority),
		TeamID:        msg.TeamID,
		AssignedToID:  msg.AssignedToID,
		MatchedRules:  []string(msg.MatchedRules),
	}
	if body.Tags == nil {
		body.Tags = []string{}
	}
	if body.Priority == "" {
		body.Priority = string(models.MessagePriorityNormal)
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		logger.Error("failed to marshal message webhook body", "message_id", msg.ID, "error", err)
		return
	}
	h.dispatcher.DispatchJSON(0, msg.WorkspaceID, event, encoded, msg.SenderEmail)
	if len(ruleWebhooks) > 0 && msg.WorkspaceID != nil {
		h.dispatcher.DispatchJSONTo(*msg.WorkspaceID, ruleWebhooks, event, encoded)
	}
}

func (h *MessageProcessHandler) notify(msg *models.Message, form *models.Form, outcome formrules.Outcome) error {
	if h.notifier == nil || !h.notifier.IsConfigured() {
		return nil
	}

	var fields []models.MessageField
	if msg.FieldsJSON != "" {
		_ = json.Unmarshal([]byte(msg.FieldsJSON), &fields)
	}

	noun := "message"
	if form.IsCollect() {
		noun = "response"
	}
	subject := fmt.Sprintf("New %s on %s", noun, form.Name)
	if msg.Status == models.MessageStatusFlagged {
		subject = fmt.Sprintf("Flagged %s on %s", noun, form.Name)
	}

	// Answers to personal-data questions stay out of email: inboxes are
	// forwarded, synced and searched far beyond the workspace.
	mask := messages.NewPIIMask(form.Fields)
	phone := msg.SenderPhone
	if mask.Value(models.RoleSenderPhone, phone) != phone {
		phone = "" // the Phone row in Fields shows it as hidden
	}
	data := map[string]any{
		"FormName":    form.Name,
		"Collect":     form.IsCollect(),
		"SenderPhone": phone,
		"Fields":      notifiableFields(msg, fields, mask),
		"Body":        mask.Value(models.RoleBody, msg.Body),
		"Flagged":     msg.Status == models.MessageStatusFlagged,
		"SpamScore":   fmt.Sprintf("%.1f", msg.SpamScore),
		"MessageURL":  h.messageURL(msg.UUID),
	}

	return h.sendRouted(msg, form, outcome, subject, data)
}

// sendRouted notifies the form's default route, unless a rule suppressed it,
// and the recipients the rules added. It returns the last error, so a retry
// may repeat a notification that already went out; that beats one never
// arriving.
func (h *MessageProcessHandler) sendRouted(msg *models.Message, form *models.Form, outcome formrules.Outcome, subject string, data map[string]any) error {
	res := formrules.Resolved{Default: true}
	if h.directory != nil {
		resolved, err := formrules.Resolve(form, outcome, h.directory)
		if err != nil {
			return fmt.Errorf("resolve rule recipients: %w", err)
		}
		res = resolved
	}
	h.addWorkspaceName(msg, data)
	var lastErr error
	if res.Default {
		lastErr = h.sendToFormRecipients(msg, form, subject, data)
	}
	for _, to := range res.Emails {
		payload := cloneData(data)
		payload["UserName"] = "there"
		if err := h.notifier.Send(to, subject, notification.TemplateNewMessage, payload); err != nil {
			lastErr = err
		}
	}
	for _, id := range res.UserIDs {
		if err := h.notifier.SendToUser(id, subject, notification.TemplateNewMessage, cloneData(data)); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// sendToFormRecipients delivers to the form's explicit notification list, or
// to the workspace owners and admins when the list is empty. With several
// addresses it returns the last error: a retry may then repeat a notification
// to an address that already got it, which beats one never arriving.
func (h *MessageProcessHandler) sendToFormRecipients(msg *models.Message, form *models.Form, subject string, data map[string]any) error {
	h.addWorkspaceName(msg, data)
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

func (h *MessageProcessHandler) addWorkspaceName(msg *models.Message, data map[string]any) {
	if h.workspaceRepo == nil || msg.WorkspaceID == nil {
		return
	}
	if _, ok := data["WorkspaceName"]; !ok {
		if ws, err := h.workspaceRepo.FindByID(*msg.WorkspaceID); err == nil && ws.Name != "" {
			data["WorkspaceName"] = ws.Name
		}
	}
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

func notifiableFields(msg *models.Message, fields []models.MessageField, mask messages.PIIMask) []models.MessageField {
	from := mask.Sender(msg.SenderName, msg.SenderEmail)
	if from == "" {
		from = "unknown sender"
	}
	out := []models.MessageField{{Key: "From", Value: from}}
	if msg.SenderPhone != "" {
		out = append(out, models.MessageField{Key: "Phone", Value: mask.Value(models.RoleSenderPhone, msg.SenderPhone)})
	}
	out = append(out, models.MessageField{Key: "Subject", Value: mask.Value(models.RoleSubject, msg.Subject)})

	for _, f := range fields {
		if summarizedFieldKeys[strings.ToLower(strings.TrimSpace(f.Key))] {
			continue
		}
		if len(out) >= 12 {
			break
		}
		if mask.Key(f.Key) && f.Value != "" {
			f.Value = messages.HiddenValue
		}
		out = append(out, f)
	}
	return out
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
			{Key: "From", Value: messages.NewPIIMask(form.Fields).Value(models.RoleSenderEmail, reply.FromAddr)},
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
