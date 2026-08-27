// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package twofactor

import (
	"github.com/pquerna/otp/totp"
)

// GenerateSecret creates a new TOTP secret for a user.
func GenerateSecret(email string) (string, string, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "Posta",
		AccountName: email,
	})
	if err != nil {
		return "", "", err
	}
	// Return secret and the otpauth URL (for QR code generation)
	return key.Secret(), key.URL(), nil
}

// ValidateCode verifies a TOTP code against a secret.
func ValidateCode(secret, code string) bool {
	return totp.Validate(code, secret)
}
