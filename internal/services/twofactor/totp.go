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
