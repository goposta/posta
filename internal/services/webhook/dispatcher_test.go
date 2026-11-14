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
	"bufio"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
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

// By default a webhook cannot reach private networks, and the recorded reason doesn't name the address.
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

// Redirects aren't followed: a public URL can't bounce into the internal network, and a 3xx isn't success.
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

type proxyHit struct{ requestLine, host string }

// fakeProxy records what each request asks it to reach; a CONNECT is tunnelled to upstream whatever its target.
func fakeProxy(t *testing.T, upstream string) (string, chan proxyHit) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	hits := make(chan proxyHit, 8)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go serveProxyConn(conn, upstream, hits)
		}
	}()
	return "http://" + ln.Addr().String(), hits
}

func serveProxyConn(conn net.Conn, upstream string, hits chan proxyHit) {
	defer func() { _ = conn.Close() }()
	br := bufio.NewReader(conn)
	var hit proxyHit
	length := 0
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		name, value, _ := strings.Cut(line, ":")
		switch {
		case hit.requestLine == "":
			hit.requestLine = line
		case strings.EqualFold(name, "Host"):
			hit.host = strings.TrimSpace(value)
		case strings.EqualFold(name, "Content-Length"):
			length, _ = strconv.Atoi(strings.TrimSpace(value))
		}
	}
	hits <- hit
	if !strings.HasPrefix(hit.requestLine, "CONNECT ") {
		// The body is drained so closing the socket doesn't reset the connection before the client reads the answer.
		_, _ = io.CopyN(io.Discard, br, int64(length))
		_, _ = io.WriteString(conn, "HTTP/1.1 200 OK\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")
		return
	}
	up, err := net.Dial("tcp", upstream)
	if err != nil {
		return
	}
	defer func() { _ = up.Close() }()
	_, _ = io.WriteString(conn, "HTTP/1.1 200 Connection established\r\n\r\n")
	go func() { _, _ = io.Copy(up, br) }()
	_, _ = io.Copy(conn, up)
}

var loopbackAllowed = netguard.Policy{Allowed: []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("::1/128")}}

// A proxy resolves the host itself, so a host Posta cannot resolve must not be handed to it.
func TestDispatcher_ProxyRefusesUnresolvableHost(t *testing.T) {
	proxyURL, hits := fakeProxy(t, "")
	t.Setenv("HTTP_PROXY", proxyURL)
	t.Setenv("HTTPS_PROXY", proxyURL)
	t.Setenv("NO_PROXY", "")

	configured := NewDispatcher(nil)
	configured.SetConfig(Config{ProxyURL: proxyURL})
	for name, d := range map[string]*Dispatcher{"configured proxy": configured, "environment proxy": NewDispatcher(nil)} {
		for _, target := range []string{"http://posta-webhook-test.invalid/hook", "https://posta-webhook-test.invalid/hook"} {
			if _, err := d.doRequest(target, "", []byte("{}")); err == nil {
				t.Errorf("%s, %s: delivered, want an error", name, target)
			} else if got := netguard.Describe(err); got != "the host name could not be resolved" {
				t.Errorf("%s, %s: reason %q", name, target, got)
			}
		}
	}
	if len(hits) != 0 {
		t.Errorf("the proxy was asked for %q", (<-hits).requestLine)
	}
}

// The proxy is sent the address Posta checked, not the host name it would resolve again.
func TestDispatcher_ProxyPinsCheckedAddress(t *testing.T) {
	proxyURL, hits := fakeProxy(t, "")
	d := NewDispatcher(nil)
	d.SetConfig(Config{ProxyURL: proxyURL, Network: loopbackAllowed})

	if status, err := d.doRequest("http://localhost:8080/hook?id=1", "", []byte("{}")); err != nil || status != http.StatusOK {
		t.Fatalf("status %d, err %v", status, err)
	}
	hit := <-hits
	if hit.requestLine != "POST http://127.0.0.1:8080/hook?id=1 HTTP/1.1" && hit.requestLine != "POST http://[::1]:8080/hook?id=1 HTTP/1.1" {
		t.Errorf("proxy asked for %q, want a pinned address", hit.requestLine)
	}
	if hit.host != "localhost:8080" {
		t.Errorf("Host header %q, want localhost:8080", hit.host)
	}

	d.SetConfig(Config{ProxyURL: proxyURL})
	if _, err := d.doRequest("http://localhost:8080/hook", "", []byte("{}")); !errors.Is(err, netguard.ErrBlocked) {
		t.Errorf("loopback through the proxy: want ErrBlocked, got %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("the proxy was asked for %q", (<-hits).requestLine)
	}
}

// Through CONNECT the tunnel targets the pinned address while TLS still names the webhook's host.
func TestDispatcher_ProxyPinsHTTPS(t *testing.T) {
	var sni atomic.Value
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.TLS = &tls.Config{GetConfigForClient: func(h *tls.ClientHelloInfo) (*tls.Config, error) {
		sni.Store(h.ServerName)
		return nil, nil
	}}
	srv.StartTLS()
	defer srv.Close()
	_, port, _ := net.SplitHostPort(srv.Listener.Addr().String())

	proxyURL, hits := fakeProxy(t, srv.Listener.Addr().String())
	d := NewDispatcher(nil)
	d.SetConfig(Config{ProxyURL: proxyURL, Network: loopbackAllowed})
	d.client.Transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	if status, err := d.doRequest("https://localhost:"+port+"/hook", "", []byte("{}")); err != nil || status != http.StatusOK {
		t.Fatalf("status %d, err %v", status, err)
	}
	hit := <-hits
	if hit.requestLine != "CONNECT 127.0.0.1:"+port+" HTTP/1.1" && hit.requestLine != "CONNECT [::1]:"+port+" HTTP/1.1" {
		t.Errorf("proxy asked for %q, want a pinned address", hit.requestLine)
	}
	if got, _ := sni.Load().(string); got != "localhost" {
		t.Errorf("TLS server name %q, want localhost", got)
	}
}
