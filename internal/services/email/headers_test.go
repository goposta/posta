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
	"encoding/json"
	"testing"
)

// System headers carry form-reply threading. They must survive with custom
// headers disabled (the default), or every queued reply loses its Reply-To.
func TestOutboundHeadersKeepSystemHeadersWhenCustomDisabled(t *testing.T) {
	req := &SendRequest{
		Headers:       map[string]string{"X-Custom": "1"},
		SystemHeaders: map[string]string{"Reply-To": "msg+tok@reply.example.com", "Message-ID": "<a@b>"},
	}
	got := outboundHeaders(req, false)
	if got["Reply-To"] != "msg+tok@reply.example.com" || got["Message-ID"] != "<a@b>" {
		t.Fatalf("system headers lost: %v", got)
	}
	if _, ok := got["X-Custom"]; ok {
		t.Fatal("custom headers must not be sent while the platform disables them")
	}
}

func TestOutboundHeadersFilterCustomWhenEnabled(t *testing.T) {
	req := &SendRequest{Headers: map[string]string{"X-Custom": "1", "From": "spoof@evil.test", "X-Posta-Internal": "x"}}
	got := outboundHeaders(req, true)
	if got["X-Custom"] != "1" {
		t.Fatalf("allowed custom header dropped: %v", got)
	}
	if _, ok := got["From"]; ok {
		t.Fatal("a reserved header must be filtered")
	}
	if _, ok := got["X-Posta-Internal"]; ok {
		t.Fatal("a blocked-prefix header must be filtered")
	}
}

func TestOutboundHeadersSystemWinsCaseInsensitively(t *testing.T) {
	req := &SendRequest{
		Headers:       map[string]string{"reply-to": "attacker@evil.test"},
		SystemHeaders: map[string]string{"Reply-To": "msg+tok@reply.example.com"},
	}
	got := outboundHeaders(req, true)
	if len(got) != 1 || got["Reply-To"] != "msg+tok@reply.example.com" {
		t.Fatalf("got %v, want only the system Reply-To", got)
	}
}

func TestOutboundHeadersNilWhenNothingToSend(t *testing.T) {
	if got := outboundHeaders(&SendRequest{Headers: map[string]string{"X-A": "1"}}, false); got != nil {
		t.Fatalf("got %v, want nil", got)
	}
}

// API callers must not be able to set system headers.
func TestSystemHeadersAreNotDecodedFromJSON(t *testing.T) {
	var req SendRequest
	if err := json.Unmarshal([]byte(`{"SystemHeaders":{"Reply-To":"x@evil.test"},"system_headers":{"Reply-To":"x@evil.test"}}`), &req); err != nil {
		t.Fatal(err)
	}
	if len(req.SystemHeaders) != 0 {
		t.Fatalf("SystemHeaders decoded from JSON: %v", req.SystemHeaders)
	}
}
