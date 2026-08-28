// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package messages

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/goposta/posta/internal/models"
)

func newTestService() *Service {
	return &Service{hmacKey: []byte("test-secret")}
}

func TestNonceRoundTrip(t *testing.T) {
	s := newTestService()
	ctx := context.Background()

	nonce, err := s.IssueNonce(ctx, "abc")
	if err != nil {
		t.Fatalf("IssueNonce: %v", err)
	}
	if err := s.VerifyNonce(ctx, "abc", nonce.Value, 0); err != nil {
		t.Fatalf("VerifyNonce: %v", err)
	}
}

func TestNonceRejectsWrongForm(t *testing.T) {
	s := newTestService()
	ctx := context.Background()

	nonce, _ := s.IssueNonce(ctx, "abc")
	if err := s.VerifyNonce(ctx, "xyz", nonce.Value, 0); !errors.Is(err, ErrNonceInvalid) {
		t.Fatalf("err = %v, want ErrNonceInvalid", err)
	}
}

func TestNonceRejectsTamperedSignature(t *testing.T) {
	s := newTestService()
	ctx := context.Background()

	nonce, _ := s.IssueNonce(ctx, "abc")
	parts := strings.Split(nonce.Value, ".")
	tampered := strings.Join(parts[:3], ".") + ".deadbeef"
	if err := s.VerifyNonce(ctx, "abc", tampered, 0); !errors.Is(err, ErrNonceInvalid) {
		t.Fatalf("err = %v, want ErrNonceInvalid", err)
	}
}

func TestNonceRejectsForeignKey(t *testing.T) {
	issuer := &Service{hmacKey: []byte("secret-a")}
	verifier := &Service{hmacKey: []byte("secret-b")}
	ctx := context.Background()

	nonce, _ := issuer.IssueNonce(ctx, "abc")
	if err := verifier.VerifyNonce(ctx, "abc", nonce.Value, 0); !errors.Is(err, ErrNonceInvalid) {
		t.Fatalf("err = %v, want ErrNonceInvalid", err)
	}
}

func TestNonceRejectsMalformedToken(t *testing.T) {
	s := newTestService()
	for _, token := range []string{"", "a", "a.b.c", "a.b.c.d.e"} {
		if err := s.VerifyNonce(context.Background(), "abc", token, 0); !errors.Is(err, ErrNonceInvalid) {
			t.Fatalf("token %q: err = %v, want ErrNonceInvalid", token, err)
		}
	}
}

func TestNonceExpires(t *testing.T) {
	s := newTestService()
	issued := time.Now().Add(-2 * nonceTTL).Unix()
	body := fmt.Sprintf("abc.%d.stale", issued)
	token := body + "." + sign(s.hmacKey, body)

	if err := s.VerifyNonce(context.Background(), "abc", token, 0); !errors.Is(err, ErrNonceExpired) {
		t.Fatalf("err = %v, want ErrNonceExpired", err)
	}
}

func TestNonceEnforcesMinimumFillTime(t *testing.T) {
	s := newTestService()
	ctx := context.Background()

	nonce, _ := s.IssueNonce(ctx, "abc")
	if err := s.VerifyNonce(ctx, "abc", nonce.Value, 30); !errors.Is(err, ErrNonceTooFast) {
		t.Fatalf("err = %v, want ErrNonceTooFast", err)
	}
}

func TestDedupHashIsStable(t *testing.T) {
	a := dedupHash(1, "ada@example.com", "hello")
	b := dedupHash(1, "ADA@example.com", "hello")
	if a != b {
		t.Fatal("dedup hash should be case-insensitive on the address")
	}
	if dedupHash(2, "ada@example.com", "hello") == a {
		t.Fatal("dedup hash should differ per form")
	}
	if dedupHash(1, "ada@example.com", "hello!") == a {
		t.Fatal("dedup hash should differ per body")
	}
}

func TestNewThreadTokenIsUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		token := newThreadToken()
		if token == "" {
			t.Fatal("empty thread token")
		}
		if seen[token] {
			t.Fatalf("duplicate thread token %s", token)
		}
		seen[token] = true
	}
}

func TestReplySubject(t *testing.T) {
	cases := map[string]string{
		"":             "Re: your message",
		"   ":          "Re: your message",
		"Question":     "Re: Question",
		"Re: Question": "Re: Question",
		"RE: Question": "RE: Question",
	}
	for in, want := range cases {
		if got := replySubject(in); got != want {
			t.Fatalf("replySubject(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestThreadReplyToRequiresDomain(t *testing.T) {
	s := newTestService()
	msg := testMessage("tok123")

	if got := s.ThreadReplyTo(msg); got != "" {
		t.Fatalf("without an inbound domain ThreadReplyTo should be empty, got %q", got)
	}

	s.cfg.InboundDomain = "reply.example.com"
	if got := s.ThreadReplyTo(msg); got != "msg+tok123@reply.example.com" {
		t.Fatalf("ThreadReplyTo = %q", got)
	}
}

func testMessage(token string) *models.Message {
	return &models.Message{ThreadToken: token}
}
