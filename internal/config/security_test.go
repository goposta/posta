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
