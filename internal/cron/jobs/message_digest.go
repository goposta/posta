// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package jobs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/notification"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/hibiken/asynq"
	"github.com/jkaninda/logger"
)

type digestItem struct {
	Sender  string
	Subject string
	Flagged bool
}

type MessageDigestJob struct {
	notifier      *notification.Service
	formRepo      *repositories.FormRepository
	messageRepo   *repositories.MessageRepository
	workspaceRepo *repositories.WorkspaceRepository
	appURL        string
}

func NewMessageDigestJob(
	notifier *notification.Service,
	formRepo *repositories.FormRepository,
	messageRepo *repositories.MessageRepository,
	workspaceRepo *repositories.WorkspaceRepository,
	appURL string,
) *MessageDigestJob {
	return &MessageDigestJob{
		notifier:      notifier,
		formRepo:      formRepo,
		messageRepo:   messageRepo,
		workspaceRepo: workspaceRepo,
		appURL:        strings.TrimSuffix(appURL, "/"),
	}
}

func (j *MessageDigestJob) Name() string     { return "message-digest" }
func (j *MessageDigestJob) Schedule() string { return "5 * * * *" }

func (j *MessageDigestJob) Run(_ context.Context, _ *asynq.Client) error {
	if j.notifier == nil || !j.notifier.IsConfigured() {
		return nil
	}

	now := time.Now().UTC()
	sent := 0

	sent += j.runMode(models.NotifyModeHourly, now.Add(-time.Hour), "Hourly", now)
	if now.Hour() == 8 {
		sent += j.runMode(models.NotifyModeDaily, now.Add(-24*time.Hour), "Daily", now)
	}

	logger.Info("message-digest: digests sent", "count", sent)
	return nil
}

func (j *MessageDigestJob) runMode(mode models.NotifyMode, since time.Time, label string, now time.Time) int {
	forms, err := j.formRepo.FindActiveWithDigest(mode)
	if err != nil {
		logger.Error("message-digest: failed to list forms", "mode", mode, "error", err)
		return 0
	}

	sent := 0
	for i := range forms {
		form := &forms[i]
		if form.WorkspaceID == nil {
			continue
		}

		pending, err := j.messageRepo.FindUnnotified(form.ID, since)
		if err != nil || len(pending) == 0 {
			continue
		}

		items := make([]digestItem, 0, len(pending))
		ids := make([]uint, 0, len(pending))
		for k := range pending {
			m := &pending[k]
			ids = append(ids, m.ID)
			if m.Status == models.MessageStatusFlagged && !form.NotifyOnFlagged {
				continue
			}
			if len(items) < 25 {
				items = append(items, digestItem{
					Sender:  senderLabel(m),
					Subject: m.Subject,
					Flagged: m.Status == models.MessageStatusFlagged,
				})
			}
		}

		if len(items) == 0 {
			_ = j.messageRepo.MarkNotified(ids, now)
			continue
		}

		workspaceName := ""
		if ws, err := j.workspaceRepo.FindByID(*form.WorkspaceID); err == nil {
			workspaceName = ws.Name
		}

		data := map[string]any{
			"Period":   label,
			"FormName": form.Name,
			"Total":    len(items),
			"Items":    items,
			"InboxURL": j.inboxURL(form.UUID),
		}
		if workspaceName != "" {
			data["WorkspaceName"] = workspaceName
		}
		subject := fmt.Sprintf("%s digest: %d new message(s) on %s", label, len(items), form.Name)

		if err := j.deliver(form, subject, data); err != nil {
			logger.Error("message-digest: failed to send", "form_id", form.ID, "error", err)
			continue
		}

		_ = j.messageRepo.MarkNotified(ids, now)
		sent++
	}
	return sent
}

func (j *MessageDigestJob) deliver(form *models.Form, subject string, data map[string]any) error {
	if len(form.NotifyEmails) > 0 {
		var lastErr error
		for _, to := range form.NotifyEmails {
			to = strings.TrimSpace(to)
			if to == "" {
				continue
			}
			payload := make(map[string]any, len(data)+1)
			for k, v := range data {
				payload[k] = v
			}
			payload["UserName"] = "there"
			if err := j.notifier.Send(to, subject, notification.TemplateMessageDigest, payload); err != nil {
				lastErr = err
			}
		}
		return lastErr
	}
	return j.notifier.SendToWorkspaceAdmins(*form.WorkspaceID, subject, notification.TemplateMessageDigest, data)
}

func (j *MessageDigestJob) inboxURL(formUUID string) string {
	if j.appURL == "" {
		return ""
	}
	return fmt.Sprintf("%s/messages?form=%s", j.appURL, formUUID)
}

func senderLabel(m *models.Message) string {
	switch {
	case m.SenderName != "" && m.SenderEmail != "":
		return fmt.Sprintf("%s <%s>", m.SenderName, m.SenderEmail)
	case m.SenderEmail != "":
		return m.SenderEmail
	case m.SenderName != "":
		return m.SenderName
	default:
		return "unknown"
	}
}
