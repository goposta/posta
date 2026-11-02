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

package notification

import (
	"html"
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

func renderedText(t *testing.T, name string, data map[string]any) string {
	t.Helper()
	return html.UnescapeString(renderOnly(t, name, data))
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
	out := renderedText(t, TemplateNewMessage, map[string]any{
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
	out := renderedText(t, TemplateMessageDigest, map[string]any{
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

func TestNewMessageTemplateRendersPhone(t *testing.T) {
	out := renderedText(t, TemplateNewMessage, map[string]any{
		"Subject":     "New message",
		"UserName":    "Ada",
		"FormName":    "Contact form",
		"SenderPhone": "+1 555 010 9999",
		"Fields":      []models.MessageField{{Key: "From", Value: "ada@example.com"}},
		"Body":        "hello",
	})
	if !strings.Contains(out, "+1 555 010 9999") {
		t.Fatal("rendered template is missing the phone number")
	}
	if !strings.Contains(out, `href="tel:`) {
		t.Fatal("the phone number should be a tel: link")
	}
}

func TestNewMessageTemplateOmitsPhoneWhenAbsent(t *testing.T) {
	out := renderedText(t, TemplateNewMessage, map[string]any{
		"Subject":  "New message",
		"UserName": "Ada",
		"FormName": "Contact form",
		"Fields":   []models.MessageField{{Key: "From", Value: "ada@example.com"}},
		"Body":     "hello",
	})
	if strings.Contains(out, "tel:") {
		t.Fatal("no phone row should render when no phone was submitted")
	}
}

func TestCollectFormTemplatesSayResponse(t *testing.T) {
	out := renderedText(t, TemplateNewMessage, map[string]any{
		"UserName": "Ada", "FormName": "Survey", "Collect": true,
		"Fields":     []models.MessageField{{Key: "From", Value: "•••• (hidden, view in Posta)"}},
		"MessageURL": "https://posta.test/messages/abc",
	})
	for _, want := range []string{"New response on Survey", "Open response", "hidden, view in Posta"} {
		if !strings.Contains(out, want) {
			t.Fatalf("collect notification is missing %q", want)
		}
	}
	if strings.Contains(out, "Open and reply") || strings.Contains(out, "before replying") {
		t.Fatal("a collect-mode notification must not offer to reply")
	}

	digest := renderedText(t, TemplateMessageDigest, map[string]any{
		"UserName": "Ada", "Period": "Daily", "FormName": "Survey", "Collect": true,
		"Total": 2, "InboxURL": "https://posta.test/forms/1?tab=responses",
	})
	for _, want := range []string{"Daily response digest", "2</strong> new responses", "Open responses"} {
		if !strings.Contains(digest, want) {
			t.Fatalf("collect digest is missing %q", want)
		}
	}
}
