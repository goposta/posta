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

package email

import (
	"mime"
	"net/mail"
	"regexp"
	"strings"
	"testing"
)

var encodedWordRe = regexp.MustCompile(`=\?[^?\s]+\?[QqBb]\?[^?\s]*\?=`)

// Some relays keep the whitespace between encoded-words, so encoded-words must break only at spaces.
func TestBuildMessageSubjectSplitsBetweenWords(t *testing.T) {
	for _, subject := range []string{
		"Your availability alert — new rooms found for 3 subscriptions",
		"Réservation confirmée : votre séjour à l’hôtel Côte d’Azur commence bientôt",
		"Места появились по 3 подпискам",
	} {
		raw := buildMessage("sender@example.com", []string{"rcpt@example.com"},
			subject, "", "body", nil, nil, "", "", false)

		msg, err := mail.ReadMessage(strings.NewReader(string(raw)))
		if err != nil {
			t.Fatalf("built message does not parse: %v", err)
		}
		value := msg.Header.Get("Subject")
		if got, _ := new(mime.WordDecoder).DecodeHeader(value); got != subject {
			t.Errorf("subject %q decodes as %q", subject, got)
		}

		naive := encodedWordRe.ReplaceAllStringFunc(value, func(w string) string {
			s, _ := new(mime.WordDecoder).Decode(w)
			return s
		})
		if strings.Join(strings.Fields(naive), " ") != subject {
			t.Errorf("subject %q arrives as %q\nencoded: %s", subject, naive, value)
		}
	}
}

// RFC 2047 limits lines carrying encoded-words to 76 characters, so long subjects must be folded.
func TestBuildMessageFoldsLongSubject(t *testing.T) {
	for _, subject := range []string{
		strings.TrimSpace(strings.Repeat("New rooms found for 3 subscriptions. ", 30)),
		strings.TrimSpace(strings.Repeat("Места появились по 3 подпискам. ", 30)),
	} {
		raw := buildMessage("sender@example.com", []string{"rcpt@example.com"},
			subject, "", "body", nil, nil, "", "", false)

		head, _, _ := strings.Cut(string(raw), "\r\n\r\n")
		for _, line := range strings.Split(head, "\r\n") {
			limit := 78
			if encodedWordRe.MatchString(line) {
				limit = 76
			}
			if len(line) > limit {
				t.Errorf("header line is %d characters, want <= %d: %s", len(line), limit, line)
			}
		}

		msg, err := mail.ReadMessage(strings.NewReader(string(raw)))
		if err != nil {
			t.Fatalf("built message does not parse: %v", err)
		}
		if got, _ := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject")); got != subject {
			t.Errorf("folded subject decodes as %q, want %q", got, subject)
		}
	}
}

// The default sender name must also break only between words and stay within the RFC 2047 line limit.
func TestBuildMessageDefaultSenderNameSplitsBetweenWords(t *testing.T) {
	const name = "Акме — компания, которая производит абсолютно всё"
	from := resolveSenderWithDefault("noreply@example.com", name)
	raw := buildMessage(from, []string{"rcpt@example.com"},
		"subject", "", "body", nil, nil, "", "", false)

	head, _, _ := strings.Cut(string(raw), "\r\n\r\n")
	for _, line := range strings.Split(head, "\r\n") {
		if encodedWordRe.MatchString(line) && len(line) > 76 {
			t.Errorf("header line is %d characters, want <= 76: %s", len(line), line)
		}
	}

	msg, err := mail.ReadMessage(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatalf("built message does not parse: %v", err)
	}
	addrs, err := msg.Header.AddressList("From")
	if err != nil || len(addrs) != 1 || addrs[0].Name != name {
		t.Fatalf("From parses as %v (%v), want name %q", addrs, err, name)
	}

	naive := encodedWordRe.ReplaceAllStringFunc(msg.Header.Get("From"), func(w string) string {
		s, _ := new(mime.WordDecoder).Decode(w)
		return s
	})
	if want := name + " <noreply@example.com>"; strings.Join(strings.Fields(naive), " ") != want {
		t.Errorf("From arrives as %q, want %q", naive, want)
	}
}

// UTF-8 senders are encoded only on the wire; senders stored pre-encoded by older versions still go out intact.
func TestBuildMessageEncodesNonASCIISender(t *testing.T) {
	const name = "Санаторий.ру"
	for _, from := range []string{
		`"Санаторий.ру" <noreply@sanatory.ru>`,
		"=?UTF-8?b?0KHQsNC90LDRgtC+0YDQuNC5LtGA0YM=?= <noreply@sanatory.ru>",
	} {
		raw := buildMessage(from, []string{"rcpt@example.com"},
			"subject", "", "body", nil, nil, "", "", false)

		head, _, _ := strings.Cut(string(raw), "\r\n\r\n")
		if !isASCII(head) {
			t.Errorf("from %q: header is not ASCII:\n%s", from, head)
		}
		msg, err := mail.ReadMessage(strings.NewReader(string(raw)))
		if err != nil {
			t.Fatalf("from %q: built message does not parse: %v", from, err)
		}
		addrs, err := msg.Header.AddressList("From")
		if err != nil || len(addrs) != 1 || addrs[0].Name != name || addrs[0].Address != "noreply@sanatory.ru" {
			t.Errorf("from %q: From parses as %v (%v), want %q <noreply@sanatory.ru>", from, addrs, err, name)
		}
	}
}

// Plain text that merely looks like an encoded-word must reach the reader as written.
func TestBuildMessageSubjectKeepsLiteralEncodedWordSyntax(t *testing.T) {
	const subject = "Use =?UTF-8?Q?hi?= in your template"
	raw := buildMessage("sender@example.com", []string{"rcpt@example.com"},
		subject, "", "body", nil, nil, "", "", false)

	msg, err := mail.ReadMessage(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatalf("built message does not parse: %v", err)
	}
	if got, _ := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject")); got != subject {
		t.Errorf("subject decodes as %q, want %q", got, subject)
	}
}
