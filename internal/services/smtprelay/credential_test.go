// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package smtprelay

import (
	"testing"

	"github.com/goposta/posta/internal/models"
)

func TestHashPassword(t *testing.T) {
	h1 := hashPassword("correct-horse-battery-staple")
	h2 := hashPassword("correct-horse-battery-staple")
	if h1 != h2 {
		t.Errorf("hashPassword() is not deterministic: %q != %q", h1, h2)
	}
}

func TestHashPasswordDiffers(t *testing.T) {
	h1 := hashPassword("password-one")
	h2 := hashPassword("password-two")
	if h1 == h2 {
		t.Errorf("hashPassword() produced same hash for different passwords: %q", h1)
	}
}

func TestVerifyPassword(t *testing.T) {
	cred := &models.SMTPCredential{PasswordHash: hashPassword("s3cret-pass")}

	if !VerifyPassword(cred, "s3cret-pass") {
		t.Error("VerifyPassword() = false, want true for correct password")
	}
	if VerifyPassword(cred, "wrong-pass") {
		t.Error("VerifyPassword() = true, want false for incorrect password")
	}
}
