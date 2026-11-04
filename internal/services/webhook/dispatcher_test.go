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

package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/netguard"
)

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

	d := localDispatcher()
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

	d := localDispatcher()
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

	d := localDispatcher()
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

// localDispatcher may reach the loopback test servers these tests start.
func localDispatcher() *Dispatcher {
	d := NewDispatcher(nil)
	d.SetConfig(Config{Network: netguard.Policy{AllowPrivate: true}})
	return d
}

// By default a webhook cannot reach the server's own network, and the reason
// recorded for the workspace does not name the address it tried.
func TestDispatcher_BlocksPrivateByDefault(t *testing.T) {
	var hit atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit.Store(true) }))
	defer srv.Close()

	d := NewDispatcher(nil)
	for _, target := range []string{srv.URL, "http://169.254.169.254/latest/meta-data/", "http://[::1]:9/", "http://0.0.0.0:6379/"} {
		_, err := d.doRequest(target, "", []byte("{}"))
		if !errors.Is(err, netguard.ErrBlocked) {
			t.Errorf("%s: want ErrBlocked, got %v", target, err)
			continue
		}
		reason := netguard.Describe(err)
		if strings.ContainsAny(reason, "0123456789") {
			t.Errorf("%s: reason leaks the address: %q", target, reason)
		}
	}
	if hit.Load() {
		t.Error("the loopback server was reached")
	}
}

// A redirect is not followed: a public URL cannot send the request into the
// internal network, and a 3xx is not a successful delivery.
func TestDispatcher_DoesNotFollowRedirects(t *testing.T) {
	var internalHit atomic.Bool
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { internalHit.Store(true) }))
	defer internal.Close()
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, internal.URL, http.StatusTemporaryRedirect)
	}))
	defer redirector.Close()

	d := localDispatcher()
	status, err := d.doRequest(redirector.URL, "", []byte("{}"))
	if err != nil || status != http.StatusTemporaryRedirect {
		t.Fatalf("status %d, err %v", status, err)
	}
	if internalHit.Load() {
		t.Error("the redirect was followed")
	}
}

// An allowed network is reachable while the rest stays blocked.
func TestDispatcher_AllowedNetworks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	d := NewDispatcher(nil)
	d.SetConfig(Config{Network: netguard.Policy{Allowed: []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")}}})
	if status, err := d.doRequest(srv.URL, "", []byte("{}")); err != nil || status != http.StatusOK {
		t.Errorf("allowed network: status %d, err %v", status, err)
	}
	if _, err := d.doRequest("http://10.255.255.1:9/", "", []byte("{}")); !errors.Is(err, netguard.ErrBlocked) {
		t.Errorf("other private network: want ErrBlocked, got %v", err)
	}
}
