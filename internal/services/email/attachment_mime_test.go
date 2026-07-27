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
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
	"testing"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/inbound"
)

// parseBuilt parses a message produced by buildMessage back into its bodies and
// attachments, the way a receiving MTA would.
func parseBuilt(t *testing.T, raw []byte) *inbound.ParsedEmail {
	t.Helper()
	p, err := inbound.ParseRawEmail(raw)
	if err != nil {
		t.Fatalf("built message does not parse: %v\n---\n%s", err, raw)
	}
	return p
}

// The SMTP relay hands Posta filenames that came from real mail clients, so they
// are routinely non-ASCII. A MIME header must be 7-bit; raw UTF-8 in one is what
// makes a downstream MTA reject the part as unreadable.
func TestBuildMessageEncodesNonASCIIAttachmentFilename(t *testing.T) {
	const filename = "Отчёт за июль.pdf"
	payload := []byte("%PDF-1.4\x00\x01binary\xffbytes")

	raw := buildMessage(
		"sender@example.com", []string{"rcpt@example.com"},
		"Проверка вложения", "", "Файл во вложении",
		[]models.Attachment{{
			Filename:    filename,
			ContentType: "application/pdf",
			Content:     base64.StdEncoding.EncodeToString(payload),
		}},
		nil, "", "", false,
	)

	for i, c := range raw {
		if c > 127 {
			t.Fatalf("non-ASCII byte %#x at offset %d; the message must be 7-bit clean\n---\n%s", c, i, raw)
		}
	}

	p := parseBuilt(t, raw)
	if len(p.Attachments) != 1 {
		t.Fatalf("got %d attachments, want 1", len(p.Attachments))
	}
	if got := p.Attachments[0].Filename; got != filename {
		t.Errorf("filename = %q, want %q", got, filename)
	}
	if got := string(p.Attachments[0].Content); got != string(payload) {
		t.Errorf("attachment content = %q, want %q", got, payload)
	}
}

// A filename is attacker-controlled on the relay path, so quotes and CRLF must
// not be able to break out of the header.
func TestBuildMessageFilenameCannotInjectHeaders(t *testing.T) {
	raw := buildMessage(
		"sender@example.com", []string{"rcpt@example.com"},
		"subject", "", "body",
		[]models.Attachment{{
			Filename:    "evil\r\nBcc: victim@example.com\r\n\r\nx\".pdf",
			ContentType: "text/plain",
			Content:     base64.StdEncoding.EncodeToString([]byte("data")),
		}},
		nil, "", "", false,
	)

	if strings.Contains(string(raw), "Bcc:") {
		t.Fatalf("filename injected a header into the message:\n%s", raw)
	}
	p := parseBuilt(t, raw)
	if len(p.Attachments) != 1 {
		t.Fatalf("got %d attachments, want 1", len(p.Attachments))
	}
	if string(p.Attachments[0].Content) != "data" {
		t.Errorf("attachment content = %q, want %q", p.Attachments[0].Content, "data")
	}
}

// Attachment bytes must survive base64 line-wrapping unchanged, at lengths that
// do and do not land on a line boundary.
func TestBuildMessageAttachmentBytesRoundTrip(t *testing.T) {
	for _, size := range []int{1, 57, 76, 1000, 4096} {
		payload := make([]byte, size)
		for i := range payload {
			payload[i] = byte(i % 256)
		}
		raw := buildMessage(
			"sender@example.com", []string{"rcpt@example.com"},
			"subject", "<p>hi</p>", "hi",
			[]models.Attachment{{
				Filename:    "blob.bin",
				ContentType: "application/octet-stream",
				Content:     base64.StdEncoding.EncodeToString(payload),
			}},
			nil, "", "", false,
		)

		p := parseBuilt(t, raw)
		if len(p.Attachments) != 1 {
			t.Fatalf("size %d: got %d attachments, want 1", size, len(p.Attachments))
		}
		if string(p.Attachments[0].Content) != string(payload) {
			t.Errorf("size %d: attachment bytes corrupted", size)
		}
	}

	// Every line must stay within the RFC 2045 76-character limit.
	raw := buildMessage(
		"sender@example.com", []string{"rcpt@example.com"},
		"subject", "", "hi",
		[]models.Attachment{{
			Filename:    "blob.bin",
			ContentType: "application/octet-stream",
			Content:     base64.StdEncoding.EncodeToString(make([]byte, 5000)),
		}},
		nil, "", "", false,
	)
	for _, line := range strings.Split(string(raw), "\r\n") {
		if len(line) > 76 {
			t.Fatalf("line exceeds 76 chars (%d): %q", len(line), line)
		}
	}
}

// End-to-end for the relay: a message as a mail client submits it, through the
// parser the relay uses, back out through the builder. The attachment must come
// out the far side byte-identical with its name intact.
func TestRelayRoundTripPreservesAttachment(t *testing.T) {
	const filename = "Отчёт за июль.pdf"
	payload := []byte("%PDF-1.4\x00\x01binary\xffbytes")

	encodedName := mime.BEncoding.Encode("UTF-8", filename)
	// Boundary begins with "--", as Rails' Mail gem emits.
	submitted := "From: Ирина <sender@example.com>\r\n" +
		"To: rcpt@example.com\r\n" +
		"Subject: " + mime.BEncoding.Encode("UTF-8", "Проверка вложения") + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=\"--==_mimepart_abc\"\r\n" +
		"\r\n" +
		"----==_mimepart_abc\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		base64.StdEncoding.EncodeToString([]byte("Файл во вложении")) + "\r\n" +
		"----==_mimepart_abc\r\n" +
		"Content-Type: application/pdf; name=\"" + encodedName + "\"\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"Content-Disposition: attachment; filename=\"" + encodedName + "\"\r\n" +
		"\r\n" +
		base64.StdEncoding.EncodeToString(payload) + "\r\n" +
		"----==_mimepart_abc--\r\n"

	// What smtprelay's Data() does with the submitted message.
	parsed, err := inbound.ParseRawEmail([]byte(submitted))
	if err != nil {
		t.Fatalf("relay failed to parse submission: %v", err)
	}
	if len(parsed.Attachments) != 1 {
		t.Fatalf("relay parsed %d attachments, want 1", len(parsed.Attachments))
	}
	atts := make([]models.Attachment, 0, len(parsed.Attachments))
	for _, a := range parsed.Attachments {
		atts = append(atts, models.Attachment{
			Filename:    a.Filename,
			ContentType: a.ContentType,
			Content:     base64.StdEncoding.EncodeToString(a.Content),
		})
	}

	relayed := buildMessage(
		"sender@example.com", []string{"rcpt@example.com"},
		parsed.Subject, parsed.HTMLBody, parsed.TextBody, atts, nil, "", "", false,
	)

	out := parseBuilt(t, relayed)
	if len(out.Attachments) != 1 {
		t.Fatalf("relayed message has %d attachments, want 1", len(out.Attachments))
	}
	if got := out.Attachments[0].Filename; got != filename {
		t.Errorf("filename = %q, want %q", got, filename)
	}
	if string(out.Attachments[0].Content) != string(payload) {
		t.Errorf("attachment bytes corrupted through the relay")
	}
	if out.TextBody != "Файл во вложении" {
		t.Errorf("text body = %q, want %q", out.TextBody, "Файл во вложении")
	}
}

// The message must be a real multipart/mixed, not a flat body with the MIME
// structure inlined as text.
func TestBuildMessageWithAttachmentIsMultipart(t *testing.T) {
	raw := buildMessage(
		"sender@example.com", []string{"rcpt@example.com"},
		"subject", "", "body text",
		[]models.Attachment{{
			Filename:    "a.txt",
			ContentType: "text/plain",
			Content:     base64.StdEncoding.EncodeToString([]byte("file data")),
		}},
		nil, "", "", false,
	)

	msg, err := mail.ReadMessage(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	mt, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("ParseMediaType: %v", err)
	}
	if mt != "multipart/mixed" {
		t.Fatalf("Content-Type = %q, want multipart/mixed", mt)
	}

	mr := multipart.NewReader(msg.Body, params["boundary"])
	var parts int
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("NextPart: %v", err)
		}
		if _, err := io.ReadAll(part); err != nil {
			t.Fatalf("read part: %v", err)
		}
		parts++
	}
	if parts != 2 {
		t.Fatalf("got %d parts, want 2 (body + attachment)", parts)
	}
}
