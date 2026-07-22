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

package config

import (
	"fmt"
	"strings"

	"github.com/jkaninda/logger"
)

const (
	// MinJWTSecretLength is the shortest signing secret considered sound. The
	// secret signs dashboard sessions, tracking-link HMACs, and the email
	// stamper, so a guessable value forges all three. Falling below this warns
	// rather than blocks; see jwtSecretProblem for why.
	MinJWTSecretLength = 32

	// MinAdminPasswordLength is the shortest seeded admin password accepted in
	// production. It only gates the initial seed, never an existing install.
	MinAdminPasswordLength = 12
)

// placeholderSecrets are values published in this repository's compose files,
// .env.example, and installation docs. They are not secrets in any deployment:
// anyone can read them, so a value matching one is treated as absent.
var placeholderSecrets = map[string]bool{
	"change-me-in-production": true,
	"change-me":               true,
	"changeme":                true,
	"admin1234":               true,
	"password":                true,
	"secret":                  true,
	"posta":                   true,
	"your-secret-key":         true,
	"supersecret":             true,
}

// isPlaceholder reports whether v is a known published placeholder. Comparison
// is case- and whitespace-insensitive so that "Change-Me-In-Production " does
// not slip through.
func isPlaceholder(v string) bool {
	return placeholderSecrets[strings.ToLower(strings.TrimSpace(v))]
}

// IsProduction reports whether the deployment declares itself production via
// POSTA_ENV. It defaults to dev, so this is opt-in: an operator who never sets
// POSTA_ENV keeps the previous quick-start behaviour and gets warnings instead
// of a refusal to boot.
func (c *Config) IsProduction() bool {
	switch strings.ToLower(strings.TrimSpace(c.Env)) {
	case "prod", "production":
		return true
	default:
		return false
	}
}

// secretProblem describes one unacceptable configuration value.
type secretProblem struct {
	envVar string
	reason string
	fatal  bool
}

func (c *Config) jwtSecretProblem() *secretProblem {
	switch {
	case strings.TrimSpace(c.JWTSecret) == "":
		return &secretProblem{"POSTA_JWT_SECRET", "is empty", true}
	case isPlaceholder(c.JWTSecret):
		return &secretProblem{"POSTA_JWT_SECRET", "is the published placeholder value, so sessions can be forged by anyone", true}
	case len(c.JWTSecret) < MinJWTSecretLength:
		return &secretProblem{
			"POSTA_JWT_SECRET",
			fmt.Sprintf("is shorter than the recommended %d characters and may be brute-forced offline from a captured token", MinJWTSecretLength),
			false,
		}
	}
	return nil
}

// adminPasswordProblem checks the seed password. It is not fatal at config
// time: an upgraded install has long since changed the password in-app and may
// not set the variable at all. storage.SeedAdmin re-checks it at the point
// where it would actually be used, which is the only place it can matter.
func (c *Config) adminPasswordProblem() *secretProblem {
	switch {
	case isPlaceholder(c.AdminPassword):
		return &secretProblem{"POSTA_ADMIN_PASSWORD", "is the published placeholder value", false}
	case len(c.AdminPassword) < MinAdminPasswordLength:
		return &secretProblem{"POSTA_ADMIN_PASSWORD", fmt.Sprintf("is shorter than %d characters", MinAdminPasswordLength), false}
	}
	return nil
}

// securityProblems collects every unacceptable value in one pass so an operator
// fixing their configuration sees the whole list rather than one item per boot.
func (c *Config) securityProblems() []secretProblem {
	var problems []secretProblem

	if p := c.jwtSecretProblem(); p != nil {
		problems = append(problems, *p)
	}
	if p := c.adminPasswordProblem(); p != nil {
		problems = append(problems, *p)
	}
	if strings.TrimSpace(c.EncryptionKey) == "" {
		problems = append(problems, secretProblem{
			"POSTA_ENCRYPTION_KEY",
			"is unset, so stored SMTP credentials fall back to base64 encoding, which is reversible",
			false,
		})
	}
	if strings.TrimSpace(c.CORSOrigins) == "*" {
		problems = append(problems, secretProblem{
			"POSTA_CORS_ORIGINS",
			"allows every origin",
			false,
		})
	}

	return problems
}

// ValidateSecurity refuses to start a production deployment whose configuration
// contains a fatal problem. Non-production deployments never fail here; they are
// reported through WarnInsecureConfig instead.
func (c *Config) ValidateSecurity() error {
	if !c.IsProduction() {
		return nil
	}
	for _, p := range c.securityProblems() {
		if p.fatal {
			return fmt.Errorf(
				"%s %s; set it to a strong random value (for example: openssl rand -hex 32). "+
					"Set POSTA_ENV=dev to run without this check outside production",
				p.envVar, p.reason)
		}
	}
	return nil
}

// WarnInsecureConfig logs every remaining problem after the logger exists. In
// production the fatal ones have already stopped the boot, so this reports the
// advisory remainder; outside production it reports everything, which is how an
// operator learns what to fix before setting POSTA_ENV=production.
func (c *Config) WarnInsecureConfig() {
	for _, p := range c.securityProblems() {
		if p.fatal && c.IsProduction() {
			continue
		}
		msg := p.envVar + " " + p.reason
		if p.fatal {
			logger.Warn("insecure configuration: "+msg+" — this will refuse to start when POSTA_ENV=production", "env", p.envVar)
			continue
		}
		logger.Warn("insecure configuration: "+msg, "env", p.envVar)
	}
}

// ValidateAdminSeedPassword reports whether the configured seed password is fit
// to create the first admin user on a production deployment. storage.SeedAdmin
// calls this only when it is about to create that user, so an existing install
// whose password was changed in-app is unaffected.
func (c *Config) ValidateAdminSeedPassword() error {
	if !c.IsProduction() {
		return nil
	}
	if p := c.adminPasswordProblem(); p != nil {
		return fmt.Errorf(
			"refusing to seed the initial admin account: %s %s; "+
				"set POSTA_ADMIN_PASSWORD to a strong value before first start",
			p.envVar, p.reason)
	}
	return nil
}
