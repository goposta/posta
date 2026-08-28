// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package worker

import (
	"testing"

	"github.com/goposta/posta/internal/models"
)

func TestShouldNotify(t *testing.T) {
	cases := []struct {
		name   string
		form   models.Form
		status models.MessageStatus
		want   bool
	}{
		{
			"received with immediate notify",
			models.Form{NotifyEnabled: true, NotifyMode: models.NotifyModeImmediate, NotifyOnFlagged: true},
			models.MessageStatusReceived, true,
		},
		{
			"notify disabled",
			models.Form{NotifyEnabled: false, NotifyMode: models.NotifyModeImmediate},
			models.MessageStatusReceived, false,
		},
		{
			"digest mode defers to the cron job",
			models.Form{NotifyEnabled: true, NotifyMode: models.NotifyModeHourly},
			models.MessageStatusReceived, false,
		},
		{
			"quarantined never notifies",
			models.Form{NotifyEnabled: true, NotifyMode: models.NotifyModeImmediate, NotifyOnFlagged: true},
			models.MessageStatusQuarantined, false,
		},
		{
			"flagged notifies when opted in",
			models.Form{NotifyEnabled: true, NotifyMode: models.NotifyModeImmediate, NotifyOnFlagged: true},
			models.MessageStatusFlagged, true,
		},
		{
			"flagged skipped when opted out",
			models.Form{NotifyEnabled: true, NotifyMode: models.NotifyModeImmediate, NotifyOnFlagged: false},
			models.MessageStatusFlagged, false,
		},
	}

	h := &MessageProcessHandler{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg := &models.Message{Status: tc.status}
			form := tc.form
			if got := h.shouldNotify(msg, &form); got != tc.want {
				t.Fatalf("shouldNotify = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDisplaySender(t *testing.T) {
	cases := []struct {
		msg  models.Message
		want string
	}{
		{models.Message{SenderName: "Ada", SenderEmail: "ada@example.com"}, "Ada <ada@example.com>"},
		{models.Message{SenderEmail: "ada@example.com"}, "ada@example.com"},
		{models.Message{SenderName: "Ada"}, "Ada"},
		{models.Message{}, "unknown sender"},
	}
	for _, tc := range cases {
		msg := tc.msg
		if got := displaySender(&msg); got != tc.want {
			t.Fatalf("displaySender(%+v) = %q, want %q", tc.msg, got, tc.want)
		}
	}
}

func TestNotifiableFieldsDropsBodyAndCaps(t *testing.T) {
	msg := &models.Message{SenderEmail: "ada@example.com", Subject: "Hi"}
	fields := []models.MessageField{{Key: "message", Value: "long body"}, {Key: "company", Value: "Acme"}}
	for i := 0; i < 20; i++ {
		fields = append(fields, models.MessageField{Key: "extra", Value: "x"})
	}

	got := notifiableFields(msg, fields)
	if len(got) > 12 {
		t.Fatalf("got %d fields, want at most 12", len(got))
	}
	for _, f := range got {
		if f.Key == "message" {
			t.Fatal("the body field must not be duplicated into the field table")
		}
	}
	if got[0].Key != "From" || got[1].Key != "Subject" {
		t.Fatalf("unexpected leading fields: %+v", got[:2])
	}
}

func TestMessageIsSpam(t *testing.T) {
	cases := map[models.MessageStatus]bool{
		models.MessageStatusReceived:    false,
		models.MessageStatusFlagged:     false,
		models.MessageStatusQuarantined: true,
		models.MessageStatusRejected:    true,
	}
	for status, want := range cases {
		msg := models.Message{Status: status}
		if got := msg.IsSpam(); got != want {
			t.Fatalf("IsSpam(%s) = %v, want %v", status, got, want)
		}
	}
}

func TestNotifiableFieldsIncludesPhoneAndDropsSummarizedAliases(t *testing.T) {
	msg := &models.Message{
		SenderName:  "Ada",
		SenderEmail: "ada@example.com",
		SenderPhone: "+1 555 010 9999",
		Subject:     "Hi",
	}
	fields := []models.MessageField{
		{Key: "name", Value: "Ada"},
		{Key: "email", Value: "ada@example.com"},
		{Key: "phone", Value: "+1 555 010 9999"},
		{Key: "subject", Value: "Hi"},
		{Key: "message", Value: "long body"},
		{Key: "company", Value: "Acme"},
	}

	got := notifiableFields(msg, fields)

	keys := make([]string, 0, len(got))
	for _, f := range got {
		keys = append(keys, f.Key)
	}
	want := []string{"From", "Phone", "Subject", "company"}
	if len(keys) != len(want) {
		t.Fatalf("keys = %v, want %v", keys, want)
	}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("keys = %v, want %v", keys, want)
		}
	}
	if got[1].Value != "+1 555 010 9999" {
		t.Fatalf("phone value = %q", got[1].Value)
	}
}

func TestNotifiableFieldsOmitsPhoneRowWhenAbsent(t *testing.T) {
	msg := &models.Message{SenderEmail: "ada@example.com", Subject: "Hi"}
	for _, f := range notifiableFields(msg, nil) {
		if f.Key == "Phone" {
			t.Fatal("a Phone row should not appear when no phone was submitted")
		}
	}
}
