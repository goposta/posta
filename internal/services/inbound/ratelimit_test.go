// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package inbound

import (
	"testing"
	"time"
)

func TestRateLimiterDisabled(t *testing.T) {
	l := NewIPRateLimiter(0, time.Second)
	for i := 0; i < 100; i++ {
		if !l.Allow("1.2.3.4") {
			t.Fatalf("disabled limiter denied request #%d", i)
		}
	}
}

func TestRateLimiterAllowsThenDenies(t *testing.T) {
	l := NewIPRateLimiter(3, time.Second)
	for i := 0; i < 3; i++ {
		if !l.Allow("1.2.3.4") {
			t.Fatalf("denied request #%d within budget", i)
		}
	}
	if l.Allow("1.2.3.4") {
		t.Fatalf("expected deny after exhausting budget")
	}
	// different IP still allowed
	if !l.Allow("5.6.7.8") {
		t.Fatalf("unrelated IP denied")
	}
}

func TestRateLimiterResetsAfterWindow(t *testing.T) {
	l := NewIPRateLimiter(1, 30*time.Millisecond)
	if !l.Allow("x") {
		t.Fatal("first should succeed")
	}
	if l.Allow("x") {
		t.Fatal("second should deny")
	}
	time.Sleep(60 * time.Millisecond)
	if !l.Allow("x") {
		t.Fatal("should reset after window")
	}
}
