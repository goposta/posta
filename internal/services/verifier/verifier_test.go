/*
 * Copyright 2026 Jonas Kaninda
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 */

package verifier

import (
	"encoding/json"
	"testing"
)

func TestDecide(t *testing.T) {
	cases := []struct {
		name                              string
		syntaxOK, disposable, role, hasMX bool
		wantStatus                        Status
		wantScore                         int
	}{
		{"bad syntax", false, false, false, false, StatusInvalid, 0},
		{"disposable beats everything else", true, true, true, true, StatusDisposable, 10},
		{"no mx", true, false, false, false, StatusInvalid, 0},
		{"role account with mx", true, false, true, true, StatusRisky, 60},
		{"clean valid", true, false, false, true, StatusValid, 90},
		{"disposable even without mx", true, true, false, false, StatusDisposable, 10},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, score, _ := decide(c.syntaxOK, c.disposable, c.role, c.hasMX)
			if status != c.wantStatus || score != c.wantScore {
				t.Fatalf("decide() = (%s, %d), want (%s, %d)", status, score, c.wantStatus, c.wantScore)
			}
		})
	}
}

func TestIsDisposable(t *testing.T) {
	if !isDisposable("Mailinator.com") {
		t.Error("expected mailinator.com to be disposable (case-insensitive)")
	}
	if isDisposable("gmail.com") {
		t.Error("gmail.com should not be disposable")
	}
}

func TestIsRoleAccount(t *testing.T) {
	for _, local := range []string{"info", "ADMIN", "no-reply", "support"} {
		if !isRoleAccount(local) {
			t.Errorf("expected %q to be a role account", local)
		}
	}
	if isRoleAccount("jonas") {
		t.Error("jonas should not be a role account")
	}
}

func TestCacheKeys(t *testing.T) {
	if got := addrKey("a@b.com"); got != "verify:addr:a@b.com" {
		t.Errorf("addrKey = %q", got)
	}
	if got := mxKey("B.com"); got != "verify:mx:b.com" {
		t.Errorf("mxKey = %q, want lowercased", got)
	}
}

func TestResultJSONRoundTrip(t *testing.T) {
	r := &Result{
		Email:  "user@example.com",
		Status: StatusValid,
		Score:  90,
		Checks: Checks{Syntax: true, MX: true, SMTP: "skipped"},
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back Result
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.Status != StatusValid || back.Score != 90 || !back.Checks.MX {
		t.Fatalf("round-trip mismatch: %+v", back)
	}
}
