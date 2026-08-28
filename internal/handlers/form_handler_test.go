// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package handlers

import (
	"strings"
	"testing"

	"github.com/goposta/posta/internal/models"
)

func TestFormSlug(t *testing.T) {
	cases := map[string]string{
		"Contact Form":           "contact-form",
		"  Support!!  ":          "support",
		"Ünïcödé Näme":           "n-c-d-n-me",
		"already-a-slug":         "already-a-slug",
		"!!!":                    "",
		"":                       "",
		strings.Repeat("a", 100): strings.Repeat("a", 60),
	}
	for in, want := range cases {
		if got := formSlug(in); got != want {
			t.Fatalf("formSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGeneratePublicKey(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		key, err := generatePublicKey()
		if err != nil {
			t.Fatalf("generatePublicKey: %v", err)
		}
		if len(key) != publicKeyLength {
			t.Fatalf("key length = %d, want %d", len(key), publicKeyLength)
		}
		if seen[key] {
			t.Fatalf("duplicate key generated: %s", key)
		}
		seen[key] = true
		for _, r := range key {
			if !strings.ContainsRune(publicKeyAlphabet, r) {
				t.Fatalf("key %q contains an out-of-alphabet rune %q", key, r)
			}
		}
	}
}

func TestNormalizeOrigins(t *testing.T) {
	got, err := normalizeOrigins([]string{" https://Example.com/ ", "", "*"})
	if err != nil {
		t.Fatalf("normalizeOrigins: %v", err)
	}
	if len(got) != 2 || got[0] != "https://example.com" || got[1] != "*" {
		t.Fatalf("got %v", got)
	}

	if _, err := normalizeOrigins([]string{"example.com"}); err == nil {
		t.Fatal("a scheme-less origin should be rejected")
	}
}

func TestNormalizeEmails(t *testing.T) {
	got, err := normalizeEmails([]string{"Ada <ada@example.com>", " bob@example.com "})
	if err != nil {
		t.Fatalf("normalizeEmails: %v", err)
	}
	if len(got) != 2 || got[0] != "ada@example.com" || got[1] != "bob@example.com" {
		t.Fatalf("got %v", got)
	}

	if _, err := normalizeEmails([]string{"nope"}); err == nil {
		t.Fatal("an invalid address should be rejected")
	}

	many := make([]string, 11)
	for i := range many {
		many[i] = "a@example.com"
	}
	if _, err := normalizeEmails(many); err == nil {
		t.Fatal("more than 10 recipients should be rejected")
	}
}

func TestNormalizeNotifyMode(t *testing.T) {
	cases := map[string]models.NotifyMode{
		"hourly":    models.NotifyModeHourly,
		"DAILY":     models.NotifyModeDaily,
		"off":       models.NotifyModeOff,
		"immediate": models.NotifyModeImmediate,
		"nonsense":  models.NotifyModeImmediate,
		"":          models.NotifyModeImmediate,
	}
	for in, want := range cases {
		if got := normalizeNotifyMode(in); got != want {
			t.Fatalf("normalizeNotifyMode(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestIsHTTPURL(t *testing.T) {
	cases := map[string]bool{
		"https://example.com/thanks": true,
		"http://example.com":         true,
		"javascript:alert(1)":        false,
		"/relative":                  false,
		"ftp://example.com":          false,
	}
	for in, want := range cases {
		if got := isHTTPURL(in); got != want {
			t.Fatalf("isHTTPURL(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestBuildHTMLSnippetIncludesHoneypot(t *testing.T) {
	form := &models.Form{Name: "Contact", HoneypotField: "_trap"}
	snippet := buildHTMLSnippet("https://posta.test/api/v1/f/abc", form)
	if !strings.Contains(snippet, `name="_trap"`) {
		t.Fatalf("snippet is missing the honeypot field:\n%s", snippet)
	}
	if !strings.Contains(snippet, `aria-hidden="true"`) {
		t.Fatal("the honeypot must be hidden from screen readers")
	}
	if !strings.Contains(snippet, "https://posta.test/api/v1/f/abc") {
		t.Fatal("snippet is missing the endpoint")
	}
}

func TestFormHelpersOnZeroValueForm(t *testing.T) {
	form := &models.Form{}
	if form.Honeypot() != models.DefaultHoneypotField {
		t.Fatalf("Honeypot() = %q", form.Honeypot())
	}
	if form.BodyLimit() != models.DefaultMaxBodyBytes {
		t.Fatalf("BodyLimit() = %d", form.BodyLimit())
	}
	if form.FieldLimit() != models.DefaultMaxFields {
		t.Fatalf("FieldLimit() = %d", form.FieldLimit())
	}
}

func TestMessageCanReply(t *testing.T) {
	cases := []struct {
		name string
		msg  models.Message
		want bool
	}{
		{"no sender", models.Message{Status: models.MessageStatusReceived}, false},
		{"rejected", models.Message{SenderEmail: "a@b.com", Status: models.MessageStatusRejected}, false},
		{"received", models.Message{SenderEmail: "a@b.com", Status: models.MessageStatusReceived}, true},
		{"flagged", models.Message{SenderEmail: "a@b.com", Status: models.MessageStatusFlagged}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.msg.CanReply(); got != tc.want {
				t.Fatalf("CanReply() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTruncateBody(t *testing.T) {
	if got := truncateBody("hello", 10); got != "hello" {
		t.Fatalf("got %q", got)
	}
	got := truncateBody(strings.Repeat("a", 50), 10)
	if len([]rune(got)) != 11 || !strings.HasSuffix(got, "…") {
		t.Fatalf("got %q", got)
	}
}
