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
	"errors"
	"fmt"
	"net/textproto"
	"testing"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/email"
	"github.com/hibiken/asynq"
)

// smtpErr mimics how email.sendViaClient wraps net/smtp's *textproto.Error for RCPT/MAIL/DATA failures.
func smtpErr(stage, recipient string, code int) error {
	inner := fmt.Errorf("SMTP %s failed: %w", stage, &textproto.Error{Code: code, Msg: "5.1.1 rejected"})
	return &email.SendError{Stage: stage, Recipient: recipient, Code: code, Msg: "5.1.1 rejected", Err: inner}
}

func TestPermanentRejection(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"550 at RCPT TO", smtpErr("RCPT TO", "dev-6@jkaninda.dev", 550), true},
		{"554 at RCPT TO", smtpErr("RCPT TO", "x@example.com", 554), true},
		{"450 transient at RCPT TO", smtpErr("RCPT TO", "x@example.com", 450), false},
		{"550 at MAIL FROM (sender-side)", smtpErr("MAIL FROM", "", 550), false},
		{"550 at DATA (content/policy)", smtpErr("DATA", "", 550), false},
		{"plain connection error", errors.New("dial tcp: connection refused"), false},
		{"wrapped send error", fmt.Errorf("SMTP send failed: %w", smtpErr("RCPT TO", "x@example.com", 550)), true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			se, ok := permanentRejection(c.err)
			if ok != c.want {
				t.Fatalf("permanentRejection() ok = %v, want %v", ok, c.want)
			}
			if ok && se == nil {
				t.Fatalf("permanentRejection() returned ok with nil SendError")
			}
			if !ok && se != nil {
				t.Fatalf("permanentRejection() returned !ok with non-nil SendError")
			}
		})
	}
}

func TestPermanentRejectionExposesRecipient(t *testing.T) {
	se, ok := permanentRejection(fmt.Errorf("SMTP send failed: %w", smtpErr("RCPT TO", "dev-6@jkaninda.dev", 550)))
	if !ok {
		t.Fatal("expected permanent rejection")
	}
	if se.Recipient != "dev-6@jkaninda.dev" {
		t.Fatalf("recipient = %q, want dev-6@jkaninda.dev", se.Recipient)
	}
	if se.Code != 550 {
		t.Fatalf("code = %d, want 550", se.Code)
	}
}

func TestServerRetryLimitReached(t *testing.T) {
	cases := []struct {
		name     string
		server   *models.SMTPServer
		attempts int
		want     bool
	}{
		{"no server", nil, 10, false},
		{"zero max_retries means no limit", &models.SMTPServer{MaxRetries: 0}, 10, false},
		{"below limit", &models.SMTPServer{MaxRetries: 3}, 2, false},
		{"at limit", &models.SMTPServer{MaxRetries: 3}, 3, true},
		{"above limit", &models.SMTPServer{MaxRetries: 3}, 13, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := serverRetryLimitReached(c.server, c.attempts); got != c.want {
				t.Fatalf("serverRetryLimitReached() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestTaskRetriesExhaustedOutsideAsynq(t *testing.T) {
	if !taskRetriesExhausted(context.Background()) {
		t.Fatalf("taskRetriesExhausted() outside an Asynq context = false, want true")
	}
}

func TestPermanentFailure(t *testing.T) {
	sendErr := errors.New("SMTP close failed: 451 try again")
	cases := []struct {
		name      string
		err       error
		exhausted bool
		want      bool
	}{
		{"retries left", sendErr, false, false},
		{"last attempt", sendErr, true, true},
		{"skip retry", fmt.Errorf("%w: %w", sendErr, asynq.SkipRetry), false, true},
		{"left for the retry worker", fmt.Errorf("%w: %w", sendErr, errResentLater), true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := permanentFailure(c.err, c.exhausted); got != c.want {
				t.Fatalf("permanentFailure() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestCountsAttempt(t *testing.T) {
	cases := []struct {
		resend  bool
		retried int
		want    bool
	}{
		{false, 0, true},
		{false, 2, true},
		{true, 0, false},
		{true, 1, true},
	}
	for _, c := range cases {
		if got := countsAttempt(c.resend, c.retried); got != c.want {
			t.Errorf("countsAttempt(resend=%v, retried=%d) = %v, want %v", c.resend, c.retried, got, c.want)
		}
	}
}

func TestRetryWorkerResends(t *testing.T) {
	enabled := &models.SMTPServer{MaxRetries: 5, Status: models.SMTPStatusEnabled}
	cases := []struct {
		name   string
		server *models.SMTPServer
		shared uint
		want   bool
	}{
		{"account server with retries", enabled, 0, true},
		{"shared server", enabled, 7, false},
		{"no max_retries", &models.SMTPServer{Status: models.SMTPStatusEnabled}, 0, false},
		{"disabled server", &models.SMTPServer{MaxRetries: 5, Status: "disabled"}, 0, false},
		{"no server", nil, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := retryWorkerResends(c.server, c.shared); got != c.want {
				t.Fatalf("retryWorkerResends() = %v, want %v", got, c.want)
			}
		})
	}
}

// The handler has no repository here, so reaching the failure path would panic.
func TestExhaustedHandlerIgnoresHandOffToRetryWorker(t *testing.T) {
	h := NewExhaustedErrorHandler(nil, nil, nil)
	task, err := newEmailSendTask(EmailSendPayload{EmailID: 1})
	if err != nil {
		t.Fatal(err)
	}
	h.HandleError(context.Background(), task, fmt.Errorf("SMTP send failed: %w", errResentLater))
}

func TestEmailSendPayloadResend(t *testing.T) {
	task, err := newEmailSendTask(EmailSendPayload{EmailID: 9, Resend: true})
	if err != nil {
		t.Fatal(err)
	}
	var p EmailSendPayload
	if err := json.Unmarshal(task.Payload(), &p); err != nil || p.EmailID != 9 || !p.Resend {
		t.Fatalf("payload = %+v, %v; want email 9 marked as a resend", p, err)
	}
	// Tasks already queued before the flag existed must still decode as first sends.
	var old EmailSendPayload
	if err := json.Unmarshal([]byte(`{"email_id":3}`), &old); err != nil || old.Resend {
		t.Fatalf("old payload = %+v, %v; want resend false", old, err)
	}
}
