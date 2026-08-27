// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package smtprelay

import (
	"errors"
	"testing"

	"github.com/emersion/go-smtp"
)

func TestMapSendError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantNil  bool
		wantCode int
	}{
		{name: "nil error accepted", err: nil, wantNil: true},
		{name: "rate limit", err: errors.New("rate_limit: too many requests"), wantCode: 452},
		{name: "domain verification", err: errors.New("domain_verification: sender domain not verified"), wantCode: 550},
		{name: "other error", err: errors.New("boom"), wantCode: 451},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapSendError(tt.err, "127.0.0.1:1234")
			if tt.wantNil {
				if got != nil {
					t.Fatalf("expected nil error, got %v", got)
				}
				return
			}
			smtpErr, ok := got.(*smtp.SMTPError)
			if !ok {
				t.Fatalf("expected *smtp.SMTPError, got %T", got)
			}
			if smtpErr.Code != tt.wantCode {
				t.Fatalf("expected code %d, got %d", tt.wantCode, smtpErr.Code)
			}
		})
	}
}
