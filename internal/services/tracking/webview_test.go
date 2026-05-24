/*
 * Copyright 2026 Jonas Kaninda
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package tracking

import (
	"strings"
	"testing"
	"time"
)

func newTestService() *Service {
	return NewService(nil, "https://posta.example.com/", []byte("test-hmac-key"))
}

func TestWebViewTokenRoundTrip(t *testing.T) {
	s := newTestService()
	const uuid = "11111111-2222-3333-4444-555555555555"

	tok := s.SignWebViewToken(uuid, time.Hour)
	got, err := s.VerifyWebViewToken(tok)
	if err != nil {
		t.Fatalf("verify returned error: %v", err)
	}
	if got != uuid {
		t.Fatalf("uuid mismatch: got %q want %q", got, uuid)
	}
}

func TestWebViewURL(t *testing.T) {
	s := newTestService()
	url := s.WebViewURL("abc")
	if !strings.HasPrefix(url, "https://posta.example.com/t/v/") {
		t.Fatalf("unexpected web view URL: %q", url)
	}
	if s.WebViewURL("") != "" {
		t.Fatalf("empty uuid should yield empty url")
	}
}

func TestWebViewTokenExpired(t *testing.T) {
	s := newTestService()
	tok := s.SignWebViewToken("abc", -time.Minute) // already expired
	if _, err := s.VerifyWebViewToken(tok); err == nil {
		t.Fatalf("expected expired token to be rejected")
	}
}

func TestWebViewTokenTampered(t *testing.T) {
	s := newTestService()
	tok := s.SignWebViewToken("abc", time.Hour)
	if _, err := s.VerifyWebViewToken(tok + "x"); err == nil {
		t.Fatalf("expected tampered signature to be rejected")
	}
}

func TestWebViewTokenWrongKind(t *testing.T) {
	s := newTestService()
	// A transactional unsubscribe token must not validate as a web-view token.
	txTok := s.SignTxUnsubscribeToken(42)
	if _, err := s.VerifyWebViewToken(txTok); err == nil {
		t.Fatalf("expected wrong-kind token to be rejected")
	}
}
