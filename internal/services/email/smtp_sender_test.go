// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package email

import (
	"errors"
	"fmt"
	"mime"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"regexp"
	"strings"
	"testing"
	"time"
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

func TestAuthPrefersPlainWhenOffered(t *testing.T) {
	a := newAuth("mail.example.com", "user", "secret")
	mech, _, err := a.Start(&smtp.ServerInfo{Name: "mail.example.com", TLS: true, Auth: []string{"LOGIN", "PLAIN"}})
	if err != nil || mech != "PLAIN" {
		t.Fatalf("mech=%q err=%v, want PLAIN", mech, err)
	}
}

func TestAuthFallsBackToPlainWhenNothingAdvertised(t *testing.T) {
	a := newAuth("mail.example.com", "user", "secret")
	mech, _, err := a.Start(&smtp.ServerInfo{Name: "mail.example.com", TLS: true})
	if err != nil || mech != "PLAIN" {
		t.Fatalf("mech=%q err=%v, want PLAIN", mech, err)
	}
}

func TestAuthUsesLoginWhenPlainNotOffered(t *testing.T) {
	a := newAuth("smtp.office365.com", "user", "secret")
	mech, _, err := a.Start(&smtp.ServerInfo{Name: "smtp.office365.com", TLS: true, Auth: []string{"LOGIN", "XOAUTH2"}})
	if err != nil || mech != "LOGIN" {
		t.Fatalf("mech=%q err=%v, want LOGIN", mech, err)
	}
	if got, _ := a.Next([]byte("Username:"), true); string(got) != "user" {
		t.Fatalf("username challenge answered with %q", got)
	}
	if got, _ := a.Next([]byte("Password:"), true); string(got) != "secret" {
		t.Fatalf("password challenge answered with %q", got)
	}
	if _, err := a.Next([]byte("Nonce:"), true); err == nil {
		t.Fatal("unexpected challenge must fail")
	}
}

func TestAuthLoginRefusesUnencryptedConnection(t *testing.T) {
	a := newAuth("smtp.office365.com", "user", "secret")
	if _, _, err := a.Start(&smtp.ServerInfo{Name: "smtp.office365.com", Auth: []string{"LOGIN"}}); err == nil {
		t.Fatal("LOGIN over plaintext must be refused")
	}
}

var messageIDRe = regexp.MustCompile(`^<[^@<>\s]+@[^@<>\s]+>$`)

// RFC 5322 requires Date and recommends Message-ID. A relay is free to forward
// a message without adding either, so buildMessage must emit them itself.
func TestBuildMessageAddsDateAndMessageID(t *testing.T) {
	raw := buildMessage(
		"Sender <sender@example.com>", []string{"rcpt@example.com"},
		"subject", "", "body",
		nil, nil, "", "", false,
	)

	msg, err := mail.ReadMessage(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatalf("built message does not parse: %v\n---\n%s", err, raw)
	}

	date, err := msg.Header.Date()
	if err != nil {
		t.Fatalf("Date header missing or unparsable: %v", err)
	}
	if time.Since(date) > time.Minute || time.Until(date) > time.Minute {
		t.Errorf("Date = %v, want within a minute of now", date)
	}

	mid := msg.Header.Get("Message-ID")
	if !messageIDRe.MatchString(mid) {
		t.Fatalf("Message-ID %q is not a well-formed msg-id", mid)
	}
	// The right-hand side should be the sender domain, so replies and bounces
	// can be correlated to the originating domain.
	if !strings.HasSuffix(mid, "@example.com>") {
		t.Errorf("Message-ID %q does not use the sender domain", mid)
	}
}

// A caller that supplies its own Date or Message-ID (custom headers) must not
// end up with a duplicate second header.
func TestBuildMessageKeepsCallerSuppliedDateAndMessageID(t *testing.T) {
	headers := map[string]string{
		"Date":       "Fri, 19 Sep 2026 22:33:25 +0000",
		"Message-ID": "<caller-fixed-id@example.org>",
	}
	raw := buildMessage(
		"sender@example.com", []string{"rcpt@example.com"},
		"subject", "", "body",
		nil, headers, "", "", false,
	)

	if got := strings.Count(string(raw), "\r\nDate:"); got != 1 {
		t.Errorf("found %d Date headers, want exactly 1\n---\n%s", got, raw)
	}
	if got := strings.Count(string(raw), "\r\nMessage-ID:"); got != 1 {
		t.Errorf("found %d Message-ID headers, want exactly 1\n---\n%s", got, raw)
	}

	msg, err := mail.ReadMessage(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatalf("built message does not parse: %v", err)
	}
	if got := msg.Header.Get("Message-ID"); got != "<caller-fixed-id@example.org>" {
		t.Errorf("Message-ID = %q, want the caller-supplied value", got)
	}
}

// Custom-header lookups are case-insensitive: a lowercase "message-id" must
// also suppress the generated one.
func TestBuildMessageHeaderLookupIsCaseInsensitive(t *testing.T) {
	raw := buildMessage(
		"sender@example.com", []string{"rcpt@example.com"},
		"subject", "", "body",
		nil, map[string]string{"message-id": "<lower@example.com>"}, "", "", false,
	)
	if strings.Count(strings.ToLower(string(raw)), "\r\nmessage-id:") != 1 {
		t.Fatalf("expected exactly one Message-ID header:\n%s", raw)
	}
}

// Two messages built back to back must not share a Message-ID, and every
// multipart message must get its own boundary instead of one hard-coded string.
func TestBuildMessageGeneratesUniqueMessageIDsAndBoundaries(t *testing.T) {
	const layout = "subject"

	var mids []string
	var boundaries []string
	for i := 0; i < 3; i++ {
		raw := buildMessage(
			"sender@example.com", []string{"rcpt@example.com"},
			layout, "<p>hi</p>", "hi", nil, nil, "", "", false,
		)
		msg, err := mail.ReadMessage(strings.NewReader(string(raw)))
		if err != nil {
			t.Fatalf("built message does not parse: %v", err)
		}
		mids = append(mids, msg.Header.Get("Message-ID"))

		_, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
		if err != nil {
			t.Fatalf("ParseMediaType: %v", err)
		}
		boundaries = append(boundaries, params["boundary"])
	}

	seen := map[string]bool{}
	for _, mid := range mids {
		if seen[mid] {
			t.Fatalf("duplicate Message-ID %q across messages", mid)
		}
		seen[mid] = true
	}
	seen = map[string]bool{}
	for _, bd := range boundaries {
		if bd == "" {
			t.Fatal("multipart message is missing a boundary parameter")
		}
		if seen[bd] {
			t.Fatalf("duplicate MIME boundary %q across messages", bd)
		}
		seen[bd] = true
		if strings.HasPrefix(bd, "posta-boundary-abc123") {
			t.Fatalf("boundary %q is the old fixed value", bd)
		}
	}
}
