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
	"errors"
	"fmt"
	"net/textproto"
	"testing"

	"github.com/goposta/posta/internal/services/email"
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
