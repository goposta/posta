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

package jobs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/formrules"
	"github.com/goposta/posta/internal/services/messages"
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
	ruleRepo      *repositories.FormRuleRepository
	directory     formrules.Directory
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

// SetRouting groups digests by the recipients routing rules chose, so a team
// is sent its own submissions rather than everyone everything.
func (j *MessageDigestJob) SetRouting(rules *repositories.FormRuleRepository, dir formrules.Directory) {
	j.ruleRepo, j.directory = rules, dir
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

		mask := messages.NewPIIMask(form.Fields)
		var rules []models.FormRule
		if j.ruleRepo != nil {
			rules, _ = j.ruleRepo.ListByForm(form.ID)
		}
		var dir formrules.Directory
		if j.directory != nil {
			dir = &formrules.CachedDirectory{Next: j.directory}
		}

		// Each recipient gets the submissions routed to them: the form's
		// default route, or an address or member a rule added.
		groups := map[digestTarget][]digestItem{}
		var order []digestTarget
		ids := make([]uint, 0, len(pending))
		for k := range pending {
			m := &pending[k]
			ids = append(ids, m.ID)
			if m.Status == models.MessageStatusFlagged && !form.NotifyOnFlagged {
				continue
			}
			item := digestItem{
				Sender:  senderLabel(m, mask),
				Subject: mask.Value(models.RoleSubject, m.Subject),
				Flagged: m.Status == models.MessageStatusFlagged,
			}
			for _, target := range j.targets(form, rules, dir, m) {
				if _, seen := groups[target]; !seen {
					order = append(order, target)
				}
				groups[target] = append(groups[target], item)
			}
		}

		if len(order) == 0 {
			_ = j.messageRepo.MarkNotified(ids, now)
			continue
		}

		workspaceName := ""
		if ws, err := j.workspaceRepo.FindByID(*form.WorkspaceID); err == nil {
			workspaceName = ws.Name
		}
		noun := "message(s)"
		if form.IsCollect() {
			noun = "response(s)"
		}

		failed := false
		for _, target := range order {
			items := groups[target]
			data := map[string]any{
				"Period":   label,
				"FormName": form.Name,
				"Collect":  form.IsCollect(),
				"Total":    len(items),
				"Items":    items,
				"InboxURL": j.inboxURL(form),
			}
			if len(items) > 25 {
				data["Items"] = items[:25]
			}
			if workspaceName != "" {
				data["WorkspaceName"] = workspaceName
			}
			subject := fmt.Sprintf("%s digest: %d new %s on %s", label, len(items), noun, form.Name)
			if err := j.send(form, target, subject, data); err != nil {
				logger.Error("message-digest: failed to send", "form_id", form.ID, "error", err)
				failed = true
			}
		}
		// A failed send keeps the submissions pending for the next run; the
		// recipients that did get theirs may then get them twice, which
		// beats someone never hearing of them.
		if failed {
			continue
		}

		_ = j.messageRepo.MarkNotified(ids, now)
		sent++
	}
	return sent
}

// digestTarget is one recipient of a digest: the form's default route, an
// address, or a member.
type digestTarget struct {
	Default bool
	Email   string
	UserID  uint
}

// targets is where one submission is reported. Without rules, that is the
// form's default route, as before.
func (j *MessageDigestJob) targets(form *models.Form, rules []models.FormRule, dir formrules.Directory, m *models.Message) []digestTarget {
	if len(rules) == 0 || dir == nil || !formrules.Routable(m.Status) {
		return []digestTarget{{Default: true}}
	}
	var out formrules.Outcome
	if m.RoutedAt != nil {
		out = formrules.Combine(rules, m.MatchedRules)
	} else {
		out = formrules.Evaluate(rules, formrules.FactsFor(m, form))
	}
	res, err := formrules.Resolve(form, out, dir)
	if err != nil {
		logger.Warn("message-digest: failed to resolve rule recipients", "form_id", form.ID, "error", err)
		return []digestTarget{{Default: true}}
	}
	var targets []digestTarget
	if res.Default {
		targets = append(targets, digestTarget{Default: true})
	}
	for _, e := range res.Emails {
		targets = append(targets, digestTarget{Email: e})
	}
	for _, id := range res.UserIDs {
		targets = append(targets, digestTarget{UserID: id})
	}
	return targets
}

func (j *MessageDigestJob) send(form *models.Form, target digestTarget, subject string, data map[string]any) error {
	switch {
	case target.Default:
		return j.deliver(form, subject, data)
	case target.Email != "":
		data["UserName"] = "there"
		return j.notifier.Send(target.Email, subject, notification.TemplateMessageDigest, data)
	default:
		return j.notifier.SendToUser(target.UserID, subject, notification.TemplateMessageDigest, data)
	}
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

// inboxURL opens the form's responses for a collect form and the inbox,
// filtered to the form, otherwise.
func (j *MessageDigestJob) inboxURL(form *models.Form) string {
	if j.appURL == "" {
		return ""
	}
	if form.IsCollect() {
		return fmt.Sprintf("%s/forms/%d?tab=responses", j.appURL, form.ID)
	}
	return fmt.Sprintf("%s/messages?form_id=%d", j.appURL, form.ID)
}

func senderLabel(m *models.Message, mask messages.PIIMask) string {
	if label := mask.Sender(m.SenderName, m.SenderEmail); label != "" {
		return label
	}
	return "unknown"
}
