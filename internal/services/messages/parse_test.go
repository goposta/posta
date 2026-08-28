// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package messages

import (
	"strings"
	"testing"

	"github.com/goposta/posta/internal/models"
)

func TestSanitizeStripsControlCharacters(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"carriage return", "hello\rworld", "helloworld"},
		{"null byte", "hel\x00lo", "hello"},
		{"keeps newline", "line1\nline2", "line1\nline2"},
		{"keeps tab", "a\tb", "a\tb"},
		{"trims", "  spaced  ", "spaced"},
		{"bell", "ding\adong", "dingdong"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Sanitize(tc.input); got != tc.want {
				t.Fatalf("Sanitize(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestSanitizeHeaderValueBlocksInjection(t *testing.T) {
	got := SanitizeHeaderValue("Jonas\r\nBcc: victim@example.com")
	if strings.ContainsAny(got, "\r\n") {
		t.Fatalf("header value still contains CR/LF: %q", got)
	}
	if got != "JonasBcc: victim@example.com" {
		t.Fatalf("unexpected sanitized value: %q", got)
	}
}

func TestSanitizeHeaderValueCapsLength(t *testing.T) {
	got := SanitizeHeaderValue(strings.Repeat("a", 500))
	if len([]rune(got)) != maxHeaderFieldRunes {
		t.Fatalf("got %d runes, want %d", len([]rune(got)), maxHeaderFieldRunes)
	}
}

func TestExtractWellKnownFields(t *testing.T) {
	fields := []models.MessageField{
		{Key: "Name", Value: "Ada Lovelace"},
		{Key: "email", Value: "ada@example.com"},
		{Key: "_subject", Value: "Question"},
		{Key: "message", Value: "How does it work?"},
	}
	got := Extract(fields)
	if got.Name != "Ada Lovelace" || got.Email != "ada@example.com" {
		t.Fatalf("unexpected sender: %+v", got)
	}
	if got.Subject != "Question" || got.Body != "How does it work?" {
		t.Fatalf("unexpected content: %+v", got)
	}
}

func TestExtractFallsBackToFirstParseableAddress(t *testing.T) {
	fields := []models.MessageField{
		{Key: "contact", Value: "grace@example.com"},
		{Key: "note", Value: "hello there"},
	}
	if got := Extract(fields); got.Email != "grace@example.com" {
		t.Fatalf("Email = %q, want grace@example.com", got.Email)
	}
}

func TestExtractJoinsNameParts(t *testing.T) {
	fields := []models.MessageField{
		{Key: "first_name", Value: "Grace"},
		{Key: "last_name", Value: "Hopper"},
	}
	if got := Extract(fields); got.Name != "Grace Hopper" {
		t.Fatalf("Name = %q, want Grace Hopper", got.Name)
	}
}

func TestExtractFallsBackToLongestValueForBody(t *testing.T) {
	fields := []models.MessageField{
		{Key: "email", Value: "a@b.com"},
		{Key: "details", Value: "this is by far the longest submitted value"},
	}
	if got := Extract(fields); got.Body != "this is by far the longest submitted value" {
		t.Fatalf("Body = %q", got.Body)
	}
}

func TestExtractWithoutEmailLeavesItEmpty(t *testing.T) {
	fields := []models.MessageField{{Key: "message", Value: "no address here"}}
	if got := Extract(fields); got.Email != "" {
		t.Fatalf("Email = %q, want empty", got.Email)
	}
}

func TestIsReserved(t *testing.T) {
	for _, key := range []string{"_gotcha", "_NONCE", "_redirect", "_captcha"} {
		if !IsReserved(key) {
			t.Fatalf("IsReserved(%q) = false, want true", key)
		}
	}
	if IsReserved("email") {
		t.Fatal("IsReserved(\"email\") = true, want false")
	}
}

func TestNormalizeEmail(t *testing.T) {
	cases := map[string]string{
		"Ada <ADA@Example.com>": "ada@example.com",
		"  ADA@EXAMPLE.COM  ":   "ada@example.com",
		"not-an-address":        "not-an-address",
	}
	for in, want := range cases {
		if got := NormalizeEmail(in); got != want {
			t.Fatalf("NormalizeEmail(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExtractPhone(t *testing.T) {
	cases := []struct {
		name   string
		fields []models.MessageField
		want   string
	}{
		{"phone", []models.MessageField{{Key: "phone", Value: "+1 555 010 9999"}}, "+1 555 010 9999"},
		{"tel alias", []models.MessageField{{Key: "Tel", Value: "0612345678"}}, "0612345678"},
		{"mobile alias", []models.MessageField{{Key: "mobile", Value: "(555) 010-9999"}}, "(555) 010-9999"},
		{"phone_number alias", []models.MessageField{{Key: "phone_number", Value: "555.010.9999"}}, "555.010.9999"},
		{"absent", []models.MessageField{{Key: "email", Value: "a@b.com"}}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Extract(tc.fields).Phone; got != tc.want {
				t.Fatalf("Phone = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNormalizePhone(t *testing.T) {
	cases := map[string]string{
		"+33 6 12 34 56 78":  "+33 6 12 34 56 78",
		"  0612345678  ":     "0612345678",
		"(555) 010-9999":     "(555) 010-9999",
		"555.010.9999 ext 4": "555.010.9999 4",
		"call me":            "",
		"":                   "",
		"tel:+15550109999":   "+15550109999",
		"+1\r\nBcc: a@b.com": "+1",
		"12  34":             "12 34",
	}
	for in, want := range cases {
		if got := NormalizePhone(in); got != want {
			t.Fatalf("NormalizePhone(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizePhoneCapsLength(t *testing.T) {
	got := NormalizePhone(strings.Repeat("1", 200))
	if len([]rune(got)) != maxPhoneRunes {
		t.Fatalf("got %d runes, want %d", len([]rune(got)), maxPhoneRunes)
	}
}

func TestNormalizePhoneStripsInjection(t *testing.T) {
	got := NormalizePhone("555\r\nBcc: victim@example.com")
	if strings.ContainsAny(got, "\r\n") || strings.Contains(got, "@") {
		t.Fatalf("phone was not sanitized: %q", got)
	}
}

func TestSummarizedKeysCoversEveryAlias(t *testing.T) {
	set := map[string]bool{}
	for _, k := range SummarizedKeys() {
		set[k] = true
	}
	for _, group := range [][]string{emailKeys, nameKeys, subjectKeys, phoneKeys} {
		for _, k := range group {
			if !set[k] {
				t.Fatalf("SummarizedKeys is missing alias %q", k)
			}
		}
	}
}
