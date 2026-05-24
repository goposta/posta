/*
 * Copyright 2026 Jonas Kaninda
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 */

package worker

import (
	"errors"
	"fmt"
	"net/textproto"
	"testing"

	"github.com/goposta/posta/internal/services/email"
)

// smtpErr builds the kind of wrapped *textproto.Error the net/smtp client
// surfaces, matching how email.sendViaClient wraps RCPT/MAIL/DATA failures.
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
