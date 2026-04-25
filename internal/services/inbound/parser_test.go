/*
 * Copyright 2026 Jonas Kaninda
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 */

package inbound

import (
	"strings"
	"testing"
)

func TestParsePlainText(t *testing.T) {
	raw := "From: alice@example.com\r\n" +
		"To: bob@example.com\r\n" +
		"Subject: Hello\r\n" +
		"Message-Id: <abc@example.com>\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"\r\n" +
		"Hi Bob.\r\n"

	p, err := ParseRawEmail([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.From != "alice@example.com" {
		t.Errorf("From = %q, want alice@example.com", p.From)
	}
	if len(p.To) != 1 || p.To[0] != "bob@example.com" {
		t.Errorf("To = %v, want [bob@example.com]", p.To)
	}
	if p.MessageID != "abc@example.com" {
		t.Errorf("MessageID = %q, want abc@example.com", p.MessageID)
	}
	if !strings.Contains(p.TextBody, "Hi Bob.") {
		t.Errorf("TextBody missing body content: %q", p.TextBody)
	}
}

func TestParseMultipartAlternative(t *testing.T) {
	raw := "From: a@x.com\r\n" +
		"To: b@x.com\r\n" +
		"Subject: Multi\r\n" +
		"Content-Type: multipart/alternative; boundary=\"xyz\"\r\n" +
		"\r\n" +
		"--xyz\r\n" +
		"Content-Type: text/plain\r\n\r\n" +
		"plain body\r\n" +
		"--xyz\r\n" +
		"Content-Type: text/html\r\n\r\n" +
		"<p>html body</p>\r\n" +
		"--xyz--\r\n"

	p, err := ParseRawEmail([]byte(raw))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !strings.Contains(p.TextBody, "plain body") {
		t.Errorf("TextBody = %q", p.TextBody)
	}
	if !strings.Contains(p.HTMLBody, "html body") {
		t.Errorf("HTMLBody = %q", p.HTMLBody)
	}
}

func TestParseMultipartWithAttachment(t *testing.T) {
	raw := "From: a@x.com\r\n" +
		"To: b@x.com\r\n" +
		"Subject: Attach\r\n" +
		"Content-Type: multipart/mixed; boundary=\"B\"\r\n" +
		"\r\n" +
		"--B\r\n" +
		"Content-Type: text/plain\r\n\r\n" +
		"hi\r\n" +
		"--B\r\n" +
		"Content-Type: application/octet-stream; name=\"x.bin\"\r\n" +
		"Content-Disposition: attachment; filename=\"x.bin\"\r\n" +
		"Content-Transfer-Encoding: base64\r\n\r\n" +
		"aGVsbG8=\r\n" +
		"--B--\r\n"

	p, err := ParseRawEmail([]byte(raw))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(p.Attachments) != 1 {
		t.Fatalf("attachments = %d, want 1", len(p.Attachments))
	}
	att := p.Attachments[0]
	if att.Filename != "x.bin" {
		t.Errorf("filename = %q, want x.bin", att.Filename)
	}
	if string(att.Content) != "hello" {
		t.Errorf("decoded content = %q, want hello", string(att.Content))
	}
}

func TestParseQuotedPrintable(t *testing.T) {
	raw := "From: a@x.com\r\n" +
		"To: b@x.com\r\n" +
		"Subject: =?UTF-8?Q?Caf=C3=A9?=\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"Content-Transfer-Encoding: quoted-printable\r\n" +
		"\r\n" +
		"Caf=C3=A9\r\n"

	p, err := ParseRawEmail([]byte(raw))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if p.Subject != "Café" {
		t.Errorf("Subject = %q, want Café", p.Subject)
	}
	if !strings.Contains(p.TextBody, "Café") {
		t.Errorf("TextBody = %q, want to contain Café", p.TextBody)
	}
}
