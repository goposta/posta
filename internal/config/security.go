// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

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

// envJWTSecret names the setting these checks report on.
const envJWTSecret = "POSTA_JWT_SECRET"

func (c *Config) jwtSecretProblem() *secretProblem {
	switch {
	case strings.TrimSpace(c.JWTSecret) == "":
		return &secretProblem{envJWTSecret, "is empty", true}
	case isPlaceholder(c.JWTSecret):
		return &secretProblem{envJWTSecret, "is the published placeholder value, so sessions can be forged by anyone", true}
	case len(c.JWTSecret) < MinJWTSecretLength:
		return &secretProblem{
			envJWTSecret,
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

// sharedSecretProblems are the values both binaries depend on.
//
// The worker signs tracking links and stamps outgoing mail with the same JWT
// secret the server uses, and it is the process that decrypts a stored SMTP
// password at send time. A worker configured differently from its server does
// not fail loudly; it produces links the server rejects and credentials it
// cannot read.
func (c *Config) sharedSecretProblems() []secretProblem {
	var problems []secretProblem

	if p := c.jwtSecretProblem(); p != nil {
		problems = append(problems, *p)
	}
	if strings.TrimSpace(c.EncryptionKey) == "" {
		problems = append(problems, secretProblem{
			"POSTA_ENCRYPTION_KEY",
			"is unset, so stored SMTP credentials fall back to base64 encoding, which is reversible",
			false,
		})
	}
	return problems
}

// securityProblems collects every unacceptable value in one pass so an operator
// fixing their configuration sees the whole list rather than one item per boot.
// This is the server's set: it includes the values only a process serving HTTP
// and seeding the first admin can act on.
func (c *Config) securityProblems() []secretProblem {
	problems := c.sharedSecretProblems()

	if p := c.adminPasswordProblem(); p != nil {
		problems = append(problems, *p)
	}
	if strings.TrimSpace(c.CORSOrigins) == "*" {
		problems = append(problems, secretProblem{
			"POSTA_CORS_ORIGINS",
			"allows every origin",
			false,
		})
	}
	if c.MessagesEnabled && c.MessagesIPRateLimit <= 0 {
		problems = append(problems, secretProblem{
			"POSTA_MESSAGES_IP_RATE_LIMIT",
			"is disabled while public form ingest is enabled, leaving the endpoint open to flooding",
			false,
		})
	}

	return problems
}

// workerProblems is the worker's set. It drops the checks the worker cannot
// act on — it seeds no admin, serves no HTTP, and exposes no form endpoint —
// and adds the ones that decide whether it will do any work at all.
func (c *Config) workerProblems() []secretProblem {
	problems := c.sharedSecretProblems()

	if c.Redis.Addr == "" && c.Redis.URL == "" {
		problems = append(problems, secretProblem{
			"POSTA_REDIS_ADDR",
			"is empty, so the worker has no queue to consume and will process nothing",
			true,
		})
	}
	if c.WorkerConcurrency <= 0 {
		problems = append(problems, secretProblem{
			"POSTA_WORKER_CONCURRENCY",
			fmt.Sprintf("is %d, so the worker would start and process nothing", c.WorkerConcurrency),
			true,
		})
	}
	if c.WorkerMaxRetries < 0 {
		problems = append(problems, secretProblem{
			"POSTA_WORKER_MAX_RETRIES",
			fmt.Sprintf("is %d; it cannot be negative", c.WorkerMaxRetries),
			true,
		})
	}
	if c.MessagesEnabled && !c.SystemSMTP.IsConfigured() {
		problems = append(problems, secretProblem{
			"POSTA_SYSTEM_SMTP_HOST",
			"is unset while web form messages are enabled, so the worker cannot deliver the notification for a submission",
			false,
		})
	}
	return problems
}

// ValidateWorker refuses to start a production worker whose configuration
// contains a fatal problem, and reports the rest. The two fatal worker checks
// are fatal everywhere, not only in production: a worker with no queue or no
// concurrency is not a degraded worker, it is a process that looks healthy and
// silently does nothing.
func (c *Config) ValidateWorker() error {
	for _, p := range c.workerProblems() {
		if !p.fatal {
			continue
		}
		if p.envVar == envJWTSecret && !c.IsProduction() {
			continue
		}
		return fmt.Errorf("%s %s", p.envVar, p.reason)
	}
	return nil
}

// WarnInsecureWorkerConfig logs the worker's advisory problems once the logger
// exists, mirroring WarnInsecureConfig for the server.
func (c *Config) WarnInsecureWorkerConfig() {
	for _, p := range c.workerProblems() {
		if p.fatal && c.IsProduction() {
			continue
		}
		msg := p.envVar + " " + p.reason
		if p.fatal {
			logger.Warn("worker configuration: "+msg, "env", p.envVar)
			continue
		}
		logger.Warn("worker configuration: "+msg, "env", p.envVar)
	}
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
