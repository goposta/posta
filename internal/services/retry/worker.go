/*
 * Copyright 2026 Jonas Kaninda
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package retry

import (
	"context"
	"sync"
	"time"

	"github.com/jkaninda/logger"
	"github.com/jkaninda/posta/internal/models"
	"github.com/jkaninda/posta/internal/services/email"
	"github.com/jkaninda/posta/internal/services/webhook"
	"github.com/jkaninda/posta/internal/storage/repositories"
)

// Worker periodically retries failed emails that haven't exceeded their
// SMTP server's max_retries limit.
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

// SetEnqueuer sets an enqueuer so retries are re-enqueued to the background worker
// instead of being sent synchronously.
func (w *Worker) SetEnqueuer(eq email.EmailEnqueuer) {
	w.enqueuer = eq
}

// OnSent sets a callback invoked after each successful retry.
func (w *Worker) OnSent(fn func()) { w.onSent = fn }

// OnFailed sets a callback invoked after each failed retry.
func (w *Worker) OnFailed(fn func()) { w.onFailed = fn }

// OnRetry sets a callback invoked for each retry attempt.
func (w *Worker) OnRetry(fn func()) { w.onRetry = fn }

// Start begins the retry loop in a background goroutine.
func (w *Worker) Start(ctx context.Context) {
	go w.run(ctx)
}

// Stop signals the worker to stop.
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
	// Get all SMTP servers that have retries enabled
	smtpServers, err := w.smtpRepo.FindAllWithRetries()
	if err != nil {
		logger.Error("retry worker: failed to load SMTP servers", "error", err)
		return
	}

	for _, server := range smtpServers {
		emails, err := w.emailRepo.FindFailedForRetry(server.UserID, server.MaxRetries)
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

	// If an enqueuer is available, re-enqueue instead of sending synchronously
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

	// Synchronous fallback
	em.RetryCount++

	if err := w.sender.Send(server, em.Sender, em.Recipients, em.Subject, em.HTMLBody, em.TextBody, nil, nil, em.ListUnsubscribeURL, em.ListUnsubscribePost); err != nil {
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
	w.dispatcher.Dispatch(em.UserID, "email.sent", em.UUID, em.Sender)
	logger.Info("retry worker: email sent successfully", "id", em.ID, "attempt", em.RetryCount)
	if w.onSent != nil {
		w.onSent()
	}
}
