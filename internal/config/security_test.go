// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package config

import (
	"strings"
	"testing"
)

// strongSecret is a stand-in for a real generated value. It is not a credential
// for anything: it exists only to be long enough and not a known placeholder.
const strongSecret = "b7f4c1a9e35d820647fbca1e9d7350af26bc4d1e8f09a3b5c7d2e4f60819a3bc"

func TestIsProduction(t *testing.T) {
	cases := map[string]bool{
		"production": true,
		"prod":       true,
		"PRODUCTION": true,
		" Prod ":     true,
		"dev":        false,
		"":           false,
		"staging":    false,
	}
	for env, want := range cases {
		c := &Config{Env: env}
		if got := c.IsProduction(); got != want {
			t.Errorf("Env=%q: IsProduction() = %v, want %v", env, got, want)
		}
	}
}

// A non-production deployment must keep booting on the shipped defaults —
// that is the quick-start path, and breaking it would be a regression.
func TestValidateSecurity_DevAcceptsDefaults(t *testing.T) {
	c := &Config{
		Env:           "dev",
		JWTSecret:     "change-me-in-production",
		AdminPassword: "admin1234",
	}
	if err := c.ValidateSecurity(); err != nil {
		t.Fatalf("dev deployment rejected: %v", err)
	}
}

// A secret that is public knowledge is no secret at all, so production refuses
// to start on one.
func TestValidateSecurity_ProductionRejectsPublicJWTSecret(t *testing.T) {
	cases := []struct {
		name   string
		secret string
	}{
		{"published placeholder", "change-me-in-production"},
		{"placeholder with different case and padding", "  Change-Me-In-Production  "},
		{"other known placeholder", "changeme"},
		{"empty", ""},
		{"whitespace only", "   "},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Config{Env: "production", JWTSecret: tc.secret, AdminPassword: strongSecret}
			err := c.ValidateSecurity()
			if err == nil {
				t.Fatalf("JWTSecret=%q accepted in production, want rejection", tc.secret)
			}
			if !strings.Contains(err.Error(), "POSTA_JWT_SECRET") {
				t.Errorf("error does not name the variable to fix: %v", err)
			}
		})
	}
}

// A short but operator-chosen secret is not published anywhere. It is reported,
// but it must not brick a running deployment on upgrade.
func TestValidateSecurity_ProductionWarnsOnShortJWTSecret(t *testing.T) {
	cases := []struct {
		name   string
		secret string
	}{
		{"short custom secret", "abc123"},
		{"one below the minimum", strings.Repeat("a", MinJWTSecretLength-1)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Config{Env: "production", JWTSecret: tc.secret, AdminPassword: strongSecret}

			if err := c.ValidateSecurity(); err != nil {
				t.Fatalf("short custom secret blocked startup, want warning only: %v", err)
			}
			if !hasProblem(c.securityProblems(), "POSTA_JWT_SECRET") {
				t.Error("short secret not reported as a problem at all")
			}
		})
	}
}

// The boundary itself must be accepted without complaint.
func TestValidateSecurity_MinimumLengthSecretIsClean(t *testing.T) {
	c := &Config{
		Env:           "production",
		JWTSecret:     strings.Repeat("a", MinJWTSecretLength),
		AdminPassword: strongSecret,
		EncryptionKey: strongSecret,
		CORSOrigins:   "https://mail.example.com",
	}
	if err := c.ValidateSecurity(); err != nil {
		t.Fatalf("secret at the minimum length rejected: %v", err)
	}
	if hasProblem(c.securityProblems(), "POSTA_JWT_SECRET") {
		t.Error("secret at the minimum length reported as a problem")
	}
}

func TestValidateSecurity_ProductionAcceptsStrongSecret(t *testing.T) {
	c := &Config{
		Env:           "production",
		JWTSecret:     strongSecret,
		AdminPassword: strongSecret,
		EncryptionKey: strongSecret,
		CORSOrigins:   "https://mail.example.com",
	}
	if err := c.ValidateSecurity(); err != nil {
		t.Fatalf("well-configured production deployment rejected: %v", err)
	}
}

// A placeholder admin password must not block startup: an upgraded install
// changed the password in-app long ago and may never set the variable. The
// check belongs at the point of seeding instead.
func TestValidateSecurity_ProductionAllowsPlaceholderAdminPassword(t *testing.T) {
	c := &Config{
		Env:           "production",
		JWTSecret:     strongSecret,
		AdminPassword: "admin1234",
	}
	if err := c.ValidateSecurity(); err != nil {
		t.Fatalf("existing install blocked by seed password: %v", err)
	}
}

// Nor must an unset encryption key or a wildcard CORS policy, which need a
// migration and a deployment decision respectively.
func TestValidateSecurity_ProductionAllowsAdvisoryProblems(t *testing.T) {
	c := &Config{
		Env:           "production",
		JWTSecret:     strongSecret,
		AdminPassword: strongSecret,
		EncryptionKey: "",
		CORSOrigins:   "*",
	}
	if err := c.ValidateSecurity(); err != nil {
		t.Fatalf("advisory problem treated as fatal: %v", err)
	}

	problems := c.securityProblems()
	for _, want := range []string{"POSTA_ENCRYPTION_KEY", "POSTA_CORS_ORIGINS"} {
		if !hasProblem(problems, want) {
			t.Errorf("%s not reported as a problem", want)
		}
	}
}

func TestValidateAdminSeedPassword(t *testing.T) {
	cases := []struct {
		name       string
		env        string
		password   string
		wantReject bool
	}{
		{"production placeholder", "production", "admin1234", true},
		{"production too short", "production", "short", true},
		{"production strong", "production", strongSecret, false},
		{"dev placeholder", "dev", "admin1234", false},
		{"dev short", "dev", "x", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Config{Env: tc.env, AdminPassword: tc.password}
			err := c.ValidateAdminSeedPassword()
			if tc.wantReject && err == nil {
				t.Fatalf("password %q accepted for seeding, want rejection", tc.password)
			}
			if !tc.wantReject && err != nil {
				t.Fatalf("password rejected unexpectedly: %v", err)
			}
			if tc.wantReject && !strings.Contains(err.Error(), "POSTA_ADMIN_PASSWORD") {
				t.Errorf("error does not name the variable to fix: %v", err)
			}
		})
	}
}

// The defaults compiled into Load must themselves be the ones the production
// check rejects. If someone changes a default, this catches a silent drift
// between the shipped value and the value the guard knows to refuse.
func TestShippedDefaultsAreRejectedInProduction(t *testing.T) {
	c := &Config{
		Env:           "production",
		JWTSecret:     "change-me-in-production",
		AdminPassword: "admin1234",
	}
	if err := c.ValidateSecurity(); err == nil {
		t.Fatal("shipped default JWT secret accepted in production")
	}
	if err := c.ValidateAdminSeedPassword(); err == nil {
		t.Fatal("shipped default admin password accepted for production seeding")
	}
}

func hasProblem(problems []secretProblem, envVar string) bool {
	for _, p := range problems {
		if p.envVar == envVar {
			return true
		}
	}
	return false
}

func workerConfig() *Config {
	return &Config{
		Env:               "production",
		JWTSecret:         strings.Repeat("a", MinJWTSecretLength),
		EncryptionKey:     strings.Repeat("b", 32),
		WorkerConcurrency: 10,
		WorkerMaxRetries:  5,
		Redis:             RedisConfig{Addr: "localhost:6379"},
	}
}

func TestValidateWorkerAcceptsAUsableConfig(t *testing.T) {
	if err := workerConfig().ValidateWorker(); err != nil {
		t.Fatalf("ValidateWorker: %v", err)
	}
}

// A worker with no queue or no concurrency is not degraded, it is a process
// that looks healthy and silently does nothing, so these refuse outside
// production too.
func TestValidateWorkerRefusesAWorkerThatCannotWork(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{"no queue", func(c *Config) { c.Redis = RedisConfig{} }, "POSTA_REDIS_ADDR"},
		{"zero concurrency", func(c *Config) { c.WorkerConcurrency = 0 }, "POSTA_WORKER_CONCURRENCY"},
		{"negative concurrency", func(c *Config) { c.WorkerConcurrency = -1 }, "POSTA_WORKER_CONCURRENCY"},
		{"negative retries", func(c *Config) { c.WorkerMaxRetries = -1 }, "POSTA_WORKER_MAX_RETRIES"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, env := range []string{"production", "dev"} {
				cfg := workerConfig()
				cfg.Env = env
				tc.mutate(cfg)

				err := cfg.ValidateWorker()
				if err == nil {
					t.Fatalf("env=%s: expected a refusal", env)
				}
				if !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("env=%s: error %q does not name %s", env, err, tc.want)
				}
			}
		})
	}
}

// A Redis URL is an alternative to the discrete address, not a missing one.
func TestValidateWorkerAcceptsARedisURL(t *testing.T) {
	cfg := workerConfig()
	cfg.Redis = RedisConfig{URL: "redis://localhost:6379/0"}

	if err := cfg.ValidateWorker(); err != nil {
		t.Fatalf("ValidateWorker: %v", err)
	}
}

// The worker seeds no admin, serves no HTTP, and exposes no form endpoint, so
// none of those should be able to stop it starting or clutter its logs.
func TestWorkerProblemsDropServerOnlyChecks(t *testing.T) {
	cfg := workerConfig()
	cfg.AdminPassword = "admin1234"
	cfg.CORSOrigins = "*"
	cfg.MessagesEnabled = true
	cfg.MessagesIPRateLimit = 0
	cfg.SystemSMTP = SystemSMTPConfig{Host: "smtp.example.com", From: "a@b.com"}

	for _, p := range cfg.workerProblems() {
		switch p.envVar {
		case "POSTA_ADMIN_PASSWORD", "POSTA_CORS_ORIGINS", "POSTA_MESSAGES_IP_RATE_LIMIT":
			t.Fatalf("worker should not be checked for %s", p.envVar)
		}
	}

	// The same values must still reach the server's list.
	var sawCORS bool
	for _, p := range cfg.securityProblems() {
		if p.envVar == "POSTA_CORS_ORIGINS" {
			sawCORS = true
		}
	}
	if !sawCORS {
		t.Fatal("the server must still be warned about a wildcard CORS policy")
	}
}

// Messages are processed by the worker, and their notification needs system
// SMTP. Advisory, not fatal: the submission is still stored.
func TestWorkerWarnsWhenMessagesHaveNoNotificationPath(t *testing.T) {
	cfg := workerConfig()
	cfg.MessagesEnabled = true

	var found bool
	for _, p := range cfg.workerProblems() {
		if p.envVar == "POSTA_SYSTEM_SMTP_HOST" {
			found = true
			if p.fatal {
				t.Fatal("a missing notification path must not stop the worker")
			}
		}
	}
	if !found {
		t.Fatal("expected a warning about the missing system SMTP")
	}
	if err := cfg.ValidateWorker(); err != nil {
		t.Fatalf("ValidateWorker should still pass: %v", err)
	}
}
