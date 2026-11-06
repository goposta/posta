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

package retry

import (
	"context"
	"sync"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/email"
	"github.com/goposta/posta/internal/services/webhook"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/logger"
)

// Worker retries failed emails that haven't exceeded their SMTP server's max_retries.
type Worker struct {
	emailRepo  *repositories.EmailRepository
	smtpRepo   *repositories.SMTPRepository
	sender     *email.SMTPSender
	dispatcher *webhook.Dispatcher
	enqueuer   email.EmailEnqueuer
	interval   time.Duration
	onSent     func()
	onFailed   func()
	onRetry    func()
	stopOnce   sync.Once
	stop       chan struct{}
}

func NewWorker(
	emailRepo *repositories.EmailRepository,
	smtpRepo *repositories.SMTPRepository,
	dispatcher *webhook.Dispatcher,
	interval time.Duration,
) *Worker {
	return &Worker{
		emailRepo:  emailRepo,
		smtpRepo:   smtpRepo,
		sender:     email.NewSMTPSender(),
		dispatcher: dispatcher,
		interval:   interval,
		stop:       make(chan struct{}),
	}
}

// SetEnqueuer makes retries go to the background worker instead of being sent synchronously.
func (w *Worker) SetEnqueuer(eq email.EmailEnqueuer) {
	w.enqueuer = eq
}

func (w *Worker) OnSent(fn func()) { w.onSent = fn }

func (w *Worker) OnFailed(fn func()) { w.onFailed = fn }

func (w *Worker) OnRetry(fn func()) { w.onRetry = fn }

func (w *Worker) Start(ctx context.Context) {
	go w.run(ctx)
}

func (w *Worker) Stop() {
	w.stopOnce.Do(func() { close(w.stop) })
}

func (w *Worker) run(ctx context.Context) {
	logger.Info("retry worker started", "interval", w.interval)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("retry worker stopped (context cancelled)")
			return
		case <-w.stop:
			logger.Info("retry worker stopped")
			return
		case <-ticker.C:
			w.processRetries()
		}
	}
}

func (w *Worker) processRetries() {
	smtpServers, err := w.smtpRepo.FindAllWithRetries()
	if err != nil {
		logger.Error("retry worker: failed to load SMTP servers", "error", err)
		return
	}

	for _, server := range smtpServers {
		var emails []models.Email
		if server.WorkspaceID != nil {
			emails, err = w.emailRepo.FindFailedForRetryByWorkspace(*server.WorkspaceID, server.MaxRetries)
		} else {
			emails, err = w.emailRepo.FindFailedForRetry(server.UserID, server.MaxRetries)
		}
		if err != nil {
			logger.Error("retry worker: failed to load failed emails", "user_id", server.UserID, "error", err)
			continue
		}

		for i := range emails {
			w.retryEmail(&emails[i], &server)
		}
	}
}

func (w *Worker) retryEmail(em *models.Email, server *models.SMTPServer) {
	if w.onRetry != nil {
		w.onRetry()
	}

	if w.enqueuer != nil {
		em.Status = models.EmailStatusQueued
		em.RetryCount++
		_ = w.emailRepo.Update(em)
		if err := w.enqueuer.EnqueueEmailSend(em.ID, ""); err != nil {
			em.Status = models.EmailStatusFailed
			em.ErrorMessage = err.Error()
			_ = w.emailRepo.Update(em)
			logger.Error("retry worker: failed to re-enqueue email", "id", em.ID, "error", err)
		} else {
			logger.Info("retry worker: email re-enqueued", "id", em.ID, "attempt", em.RetryCount)
		}
		return
	}

	em.RetryCount++

	if err := w.sender.Send(server, em.Sender, em.Recipients, em.Subject, em.HTMLBody, em.TextBody, nil, nil, em.ListUnsubscribeURL, em.ListUnsubscribeMailto, em.ListUnsubscribePost); err != nil {
		em.ErrorMessage = err.Error()
		_ = w.emailRepo.Update(em)
		logger.Debug("retry worker: email retry failed", "id", em.ID, "attempt", em.RetryCount, "error", err)
		if w.onFailed != nil {
			w.onFailed()
		}
		return
	}

	now := time.Now()
	em.Status = models.EmailStatusSent
	em.SentAt = &now
	em.ErrorMessage = ""
	_ = w.emailRepo.Update(em)
	w.dispatcher.Dispatch(em.UserID, em.WorkspaceID, "email.sent", em.UUID, em.Sender)
	logger.Info("retry worker: email sent successfully", "id", em.ID, "attempt", em.RetryCount)
	if w.onSent != nil {
		w.onSent()
	}
}
