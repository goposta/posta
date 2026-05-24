/*
 * Copyright 2026 Jonas Kaninda
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 */

package email

import (
	"errors"
	"fmt"
	"net/textproto"
	"testing"
)

func TestSmtpReply(t *testing.T) {
	// A *textproto.Error wrapped the way the net/smtp client surfaces a 550.
	rcptErr := fmt.Errorf("SMTP RCPT TO failed: %w", &textproto.Error{
		Code: 550,
		Msg:  "5.1.1 <dev-6@jkaninda.dev>: Recipient address rejected: User unknown",
	})
	code, msg := smtpReply(rcptErr)
	if code != 550 {
		t.Fatalf("smtpReply code = %d, want 550", code)
	}
	if msg == "" {
		t.Fatalf("smtpReply msg is empty, want server text")
	}

	// A plain connection error carries no SMTP reply code.
	if code, _ := smtpReply(errors.New("dial tcp: connection refused")); code != 0 {
		t.Fatalf("smtpReply code = %d for non-SMTP error, want 0", code)
	}
}

func TestSendErrorPermanent(t *testing.T) {
	cases := []struct {
		code int
		want bool
	}{
		{550, true},
		{551, true},
		{554, true},
		{450, false}, // transient
		{421, false}, // transient
		{0, false},   // connection-level
		{250, false},
	}
	for _, c := range cases {
		se := &SendError{Code: c.code, Err: errors.New("x")}
		if got := se.Permanent(); got != c.want {
			t.Errorf("SendError{Code:%d}.Permanent() = %v, want %v", c.code, got, c.want)
		}
	}
}

func TestWrapSendError(t *testing.T) {
	orig := fmt.Errorf("SMTP RCPT TO failed: %w", &textproto.Error{Code: 550, Msg: "5.1.1 rejected"})
	err := wrapSendError("RCPT TO", "user@example.com", orig)

	var se *SendError
	if !errors.As(err, &se) {
		t.Fatalf("wrapSendError result is not a *SendError")
	}
	if se.Stage != "RCPT TO" || se.Recipient != "user@example.com" || se.Code != 550 {
		t.Fatalf("unexpected SendError: %+v", se)
	}
	if !se.Permanent() {
		t.Fatalf("550 should be permanent")
	}
	// Error string must be preserved unchanged for storage/logging.
	if se.Error() != orig.Error() {
		t.Fatalf("Error() = %q, want %q", se.Error(), orig.Error())
	}
}
