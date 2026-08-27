// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package notification

import (
	"html/template"
	"strings"
	"testing"

	"github.com/goposta/posta/internal/models"
)

func renderOnly(t *testing.T, name string, data map[string]any) string {
	t.Helper()
	tmpl := template.Must(template.ParseFS(templateFS,
		"templates/base.tmpl",
		"templates/"+name+".tmpl",
	))
	var sb strings.Builder
	if err := tmpl.ExecuteTemplate(&sb, "base", data); err != nil {
		t.Fatalf("render %s: %v", name, err)
	}
	return sb.String()
}

func TestEveryRegisteredTemplateParses(t *testing.T) {
	s := &Service{templates: map[string]*template.Template{}}
	s.loadTemplates()

	for _, name := range []string{TemplateNewMessage, TemplateMessageDigest} {
		if _, ok := s.templates[name]; !ok {
			t.Fatalf("template %q is not registered in loadTemplates", name)
		}
	}
}

func TestNewMessageTemplateRenders(t *testing.T) {
	out := renderOnly(t, TemplateNewMessage, map[string]any{
		"Subject":       "New message on Contact form",
		"UserName":      "Ada",
		"FormName":      "Contact form",
		"WorkspaceName": "Acme",
		"Fields": []models.MessageField{
			{Key: "From", Value: "grace@example.com"},
			{Key: "Subject", Value: "Hello"},
		},
		"Body":       "Plain body text",
		"Flagged":    true,
		"SpamScore":  "4.5",
		"MessageURL": "https://posta.test/messages/abc",
	})

	for _, want := range []string{"Contact form", "Acme", "grace@example.com", "Plain body text", "4.5", "https://posta.test/messages/abc"} {
		if !strings.Contains(out, want) {
			t.Fatalf("rendered template is missing %q", want)
		}
	}
}

func TestNewMessageTemplateEscapesSubmittedContent(t *testing.T) {
	out := renderOnly(t, TemplateNewMessage, map[string]any{
		"Subject":  "New message",
		"UserName": "Ada",
		"FormName": "Contact form",
		"Fields": []models.MessageField{
			{Key: "name", Value: `<img src=x onerror=alert(1)>`},
		},
		"Body": `<script>alert("xss")</script>`,
	})

	if strings.Contains(out, "<script>alert") || strings.Contains(out, "<img src=x") {
		t.Fatalf("submitted content was not escaped:\n%s", out)
	}
	if !strings.Contains(out, "&lt;script&gt;") {
		t.Fatal("expected the body to be HTML-escaped")
	}
}

func TestMessageDigestTemplateRenders(t *testing.T) {
	type item struct {
		Sender  string
		Subject string
		Flagged bool
	}
	out := renderOnly(t, TemplateMessageDigest, map[string]any{
		"Subject":  "Hourly digest",
		"UserName": "Ada",
		"Period":   "Hourly",
		"FormName": "Contact form",
		"Total":    2,
		"Items": []item{
			{Sender: "a@example.com", Subject: "One", Flagged: false},
			{Sender: "b@example.com", Subject: "Two", Flagged: true},
		},
		"InboxURL": "https://posta.test/messages",
	})

	for _, want := range []string{"Hourly", "a@example.com", "b@example.com", "(flagged)", "https://posta.test/messages"} {
		if !strings.Contains(out, want) {
			t.Fatalf("rendered digest is missing %q", want)
		}
	}
}
