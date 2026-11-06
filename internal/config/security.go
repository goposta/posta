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

package config

import (
	"fmt"
	"slices"
	"strings"

	"github.com/jkaninda/logger"
)

const (
	// The JWT secret signs sessions, tracking HMACs and the stamper; being too short warns (see jwtSecretProblem).
	MinJWTSecretLength = 32

	// MinAdminPasswordLength only gates the initial production admin seed, never an existing install.
	MinAdminPasswordLength = 12
)

// placeholderSecrets are published in this repo's compose files and docs, so a match is treated as absent.
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

// Case- and whitespace-insensitive so "Change-Me-In-Production " does not slip through.
func isPlaceholder(v string) bool {
	return placeholderSecrets[strings.ToLower(strings.TrimSpace(v))]
}

// IsProduction is opt-in via POSTA_ENV, so installs that never set it get warnings instead of a boot refusal.
func (c *Config) IsProduction() bool {
	switch strings.ToLower(strings.TrimSpace(c.Env)) {
	case "prod", "production":
		return true
	default:
		return false
	}
}

type secretProblem struct {
	envVar string
	reason string
	fatal  bool
}

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

// Not fatal at config time: upgraded installs changed it in-app; storage.SeedAdmin re-checks it when seeding.
func (c *Config) adminPasswordProblem() *secretProblem {
	switch {
	case isPlaceholder(c.AdminPassword):
		return &secretProblem{"POSTA_ADMIN_PASSWORD", "is the published placeholder value", false}
	case len(c.AdminPassword) < MinAdminPasswordLength:
		return &secretProblem{"POSTA_ADMIN_PASSWORD", fmt.Sprintf("is shorter than %d characters", MinAdminPasswordLength), false}
	}
	return nil
}

// The worker shares the JWT secret and decrypts SMTP passwords, so a mismatch silently breaks links and sends.
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

// securityProblems collects every problem in one pass so operators see the whole list, not one per boot.
func (c *Config) securityProblems() []secretProblem {
	problems := c.sharedSecretProblems()

	if p := c.adminPasswordProblem(); p != nil {
		problems = append(problems, *p)
	}
	if slices.Contains(splitList(c.CORSOrigins), "*") {
		problems = append(problems, secretProblem{
			"POSTA_CORS_ORIGINS",
			"allows every origin",
			false,
		})
	}
	if len(c.TrustedProxies) == 0 {
		problems = append(problems, secretProblem{
			"POSTA_TRUSTED_PROXIES",
			"is unset, so X-Forwarded-For is trusted from any client and the IP used for rate limiting, API key allow-lists and audit logs can be spoofed",
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

// workerProblems drops server-only checks (admin seed, HTTP, forms) and adds whether the worker can work at all.
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
	if c.WorkerHealthEnabled && (c.WorkerHealthPort <= 0 || c.WorkerHealthPort > 65535) {
		problems = append(problems, secretProblem{
			"POSTA_WORKER_HEALTH_PORT",
			fmt.Sprintf("is %d, which is not a usable port, so the worker would expose no probes or metrics", c.WorkerHealthPort),
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

// A worker with no queue or concurrency is fatal everywhere: it looks healthy but silently does nothing.
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

// ValidateSecurity fails only in production; other deployments get WarnInsecureConfig warnings instead.
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

// Outside production this reports everything, which is how operators learn what to fix before going live.
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

// storage.SeedAdmin calls this only when creating the first admin, so in-app password changes are unaffected.
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
