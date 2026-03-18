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

package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jkaninda/posta/internal/models"
)

// --- signPayload tests ---

func TestSignPayload(t *testing.T) {
	secret := "test-secret"
	body := []byte(`{"event":"email.sent"}`)

	sig := signPayload(secret, body)

	// Verify independently
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	want := hex.EncodeToString(mac.Sum(nil))

	if sig != want {
		t.Fatalf("signPayload = %q, want %q", sig, want)
	}
}

func TestSignPayloadDifferentSecrets(t *testing.T) {
	body := []byte(`{"event":"email.sent"}`)
	sig1 := signPayload("secret-a", body)
	sig2 := signPayload("secret-b", body)
	if sig1 == sig2 {
		t.Fatal("different secrets should produce different signatures")
	}
}

func TestSignPayloadDifferentBodies(t *testing.T) {
	secret := "same-secret"
	sig1 := signPayload(secret, []byte(`{"event":"email.sent"}`))
	sig2 := signPayload(secret, []byte(`{"event":"email.failed"}`))
	if sig1 == sig2 {
		t.Fatal("different bodies should produce different signatures")
	}
}

// --- matchesFilters tests ---

func TestMatchesFilters_EmptyFilters(t *testing.T) {
	if !matchesFilters(nil, "user@example.com") {
		t.Error("empty filters should match any sender")
	}
	if !matchesFilters([]string{}, "user@example.com") {
		t.Error("empty slice filters should match any sender")
	}
}

func TestMatchesFilters_EmptyFrom(t *testing.T) {
	if !matchesFilters([]string{"example.com"}, "") {
		t.Error("empty from should match (no sender info to reject)")
	}
}

func TestMatchesFilters_ExactMatch(t *testing.T) {
	if !matchesFilters([]string{"user@example.com"}, "user@example.com") {
		t.Error("exact email match should pass")
	}
}

func TestMatchesFilters_CaseInsensitive(t *testing.T) {
	if !matchesFilters([]string{"User@Example.COM"}, "user@example.com") {
		t.Error("filter matching should be case-insensitive")
	}
}

func TestMatchesFilters_DomainMatch(t *testing.T) {
	if !matchesFilters([]string{"example.com"}, "anyone@example.com") {
		t.Error("domain filter should match any sender from that domain")
	}
}

func TestMatchesFilters_NoMatch(t *testing.T) {
	if matchesFilters([]string{"other.com"}, "user@example.com") {
		t.Error("non-matching domain should not match")
	}
}

func TestMatchesFilters_RFC5322Address(t *testing.T) {
	if !matchesFilters([]string{"user@example.com"}, "User Name <user@example.com>") {
		t.Error("should parse RFC 5322 addresses")
	}
}

func TestMatchesFilters_MultipleFilters(t *testing.T) {
	filters := []string{"other.com", "user@example.com"}
	if !matchesFilters(filters, "user@example.com") {
		t.Error("should match when any filter matches")
	}
	if !matchesFilters(filters, "admin@other.com") {
		t.Error("should match domain filter in list")
	}
	if matchesFilters(filters, "admin@nope.com") {
		t.Error("should not match when no filter matches")
	}
}

// --- Dispatcher delivery tests ---

func TestDispatcher_SignatureHeader(t *testing.T) {
	var receivedSig string
	var receivedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSig = r.Header.Get("X-Posta-Signature")
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		receivedBody = buf
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	secret := "webhook-secret-123"
	wh := models.Webhook{
		ID:     1,
		UserID: 1,
		URL:    srv.URL,
		Events: []string{"email.sent"},
		Secret: secret,
	}

	d := NewDispatcher(nil)
	body := []byte(`{"event":"email.sent","email_id":"abc","timestamp":"2026-01-01T00:00:00Z"}`)

	statusCode, err := d.doRequest(wh.URL, wh.Secret, body)
	if err != nil {
		t.Fatalf("doRequest error: %v", err)
	}
	if statusCode != 200 {
		t.Fatalf("expected 200, got %d", statusCode)
	}

	expectedSig := "sha256=" + signPayload(secret, body)
	if receivedSig != expectedSig {
		t.Errorf("signature = %q, want %q", receivedSig, expectedSig)
	}
	_ = receivedBody
}

func TestDispatcher_NoSignatureWhenNoSecret(t *testing.T) {
	var receivedSig string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSig = r.Header.Get("X-Posta-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := NewDispatcher(nil)
	_, err := d.doRequest(srv.URL, "", []byte(`{}`))
	if err != nil {
		t.Fatalf("doRequest error: %v", err)
	}
	if receivedSig != "" {
		t.Errorf("expected no signature header, got %q", receivedSig)
	}
}

func TestDispatcher_RetryOnFailure(t *testing.T) {
	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := NewDispatcher(nil)
	d.maxRetries = 3

	wh := models.Webhook{
		ID:     1,
		UserID: 1,
		URL:    srv.URL,
		Events: []string{"email.sent"},
		Secret: "secret",
	}

	payload := Payload{Event: "email.sent", EmailID: "test-123", Timestamp: time.Now().UTC().Format(time.RFC3339)}
	body, _ := json.Marshal(payload)

	d.deliverWithRetry(1, wh, body, "email.sent")

	if got := attempts.Load(); got != 3 {
		t.Errorf("expected 3 attempts, got %d", got)
	}
}

func TestDispatcher_SetConfig(t *testing.T) {
	d := NewDispatcher(nil)

	if d.maxRetries != defaultMaxRetries {
		t.Errorf("default maxRetries = %d, want %d", d.maxRetries, defaultMaxRetries)
	}

	d.SetConfig(Config{MaxRetries: 5, Timeout: 30 * time.Second})

	if d.maxRetries != 5 {
		t.Errorf("after SetConfig maxRetries = %d, want 5", d.maxRetries)
	}
	if d.client.Timeout != 30*time.Second {
		t.Errorf("after SetConfig timeout = %v, want 30s", d.client.Timeout)
	}
}
