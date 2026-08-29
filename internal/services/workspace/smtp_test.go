// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package workspace

import (
	"testing"

	"github.com/goposta/posta/internal/config"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/crypto"
)

func TestNormalizeEncryption(t *testing.T) {
	cases := map[string]string{
		"none":     models.EncryptionNone,
		"NONE":     models.EncryptionNone,
		"ssl":      models.EncryptionSSL,
		"SSL":      models.EncryptionSSL,
		"tls":      models.EncryptionSSL,
		"starttls": models.EncryptionSTARTTLS,
		"":         models.EncryptionSTARTTLS,
		"  ":       models.EncryptionSTARTTLS,
		"garbage":  models.EncryptionSTARTTLS,
	}
	for in, want := range cases {
		if got := NormalizeEncryption(in); got != want {
			t.Fatalf("NormalizeEncryption(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseSender(t *testing.T) {
	cases := []struct {
		in       string
		wantName string
		wantAddr string
		wantErr  bool
	}{
		{"posta@example.com", "", "posta@example.com", false},
		{"Posta <posta@example.com>", "Posta", "posta@example.com", false},
		{"  posta@example.com  ", "", "posta@example.com", false},
		{"", "", "", true},
		{"not-an-address", "", "", true},
	}
	for _, tc := range cases {
		name, addr, err := ParseSender(tc.in)
		if (err != nil) != tc.wantErr {
			t.Fatalf("ParseSender(%q) err = %v, wantErr %v", tc.in, err, tc.wantErr)
		}
		if err != nil {
			continue
		}
		if name != tc.wantName || addr != tc.wantAddr {
			t.Fatalf("ParseSender(%q) = (%q, %q), want (%q, %q)", tc.in, name, addr, tc.wantName, tc.wantAddr)
		}
	}
}

func testConfig() config.SystemSMTPConfig {
	return config.SystemSMTPConfig{
		Host:       "smtp.example.com",
		Port:       587,
		Username:   "posta",
		Password:   "s3cret",
		From:       "Posta <posta@example.com>",
		Encryption: "starttls",
	}
}

func syncedServer(cfg config.SystemSMTPConfig) *models.SMTPServer {
	return &models.SMTPServer{
		Host:       cfg.Host,
		Port:       cfg.Port,
		Username:   cfg.Username,
		Password:   cfg.Password,
		Encryption: NormalizeEncryption(cfg.Encryption),
	}
}

func TestConnectionDriftNoChange(t *testing.T) {
	cfg := testConfig()
	if got := connectionDrift(cfg, syncedServer(cfg)); len(got) != 0 {
		t.Fatalf("an in-sync row must produce no update, got %v", got)
	}
}

func TestConnectionDriftPerField(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*models.SMTPServer)
		column string
	}{
		{"host", func(s *models.SMTPServer) { s.Host = "old.example.com" }, "host"},
		{"port", func(s *models.SMTPServer) { s.Port = 25 }, "port"},
		{"username", func(s *models.SMTPServer) { s.Username = "old" }, "username"},
		{"encryption", func(s *models.SMTPServer) { s.Encryption = models.EncryptionNone }, "encryption"},
		{"password", func(s *models.SMTPServer) { s.Password = "old" }, "password"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testConfig()
			server := syncedServer(cfg)
			tc.mutate(server)

			got := connectionDrift(cfg, server)
			if len(got) != 1 {
				t.Fatalf("expected exactly one changed column, got %v", got)
			}
			if _, ok := got[tc.column]; !ok {
				t.Fatalf("expected column %q to change, got %v", tc.column, got)
			}
		})
	}
}

// A map-based Updates call bypasses GORM's BeforeSave hook, so the rotated
// password has to be encrypted here. Without it the secret lands in the
// database in the clear, and nothing else in the system would notice.
func TestConnectionDriftEncryptsRotatedPassword(t *testing.T) {
	crypto.Init("0123456789abcdef0123456789abcdef")

	cfg := testConfig()
	server := syncedServer(cfg)
	server.Password = "the-previous-one"

	got := connectionDrift(cfg, server)
	stored, ok := got["password"].(string)
	if !ok {
		t.Fatalf("password not updated: %v", got)
	}
	if stored == cfg.Password {
		t.Fatal("the rotated password was stored in plaintext")
	}
	if !crypto.IsEncrypted(stored) {
		t.Fatalf("stored password is not encrypted: %q", stored)
	}
	if plain, err := crypto.Decrypt(stored); err != nil || plain != cfg.Password {
		t.Fatalf("stored password does not decrypt to the configured one: %q, %v", plain, err)
	}
}

// An unset password must not clear the stored one: an operator may have set it
// through the dashboard and dropped it from the environment.
func TestConnectionDriftIgnoresEmptyPassword(t *testing.T) {
	cfg := testConfig()
	cfg.Password = ""
	server := syncedServer(testConfig())

	if _, ok := connectionDrift(cfg, server)["password"]; ok {
		t.Fatal("an empty configured password must not overwrite the stored one")
	}
}

func TestChangedColumnsIsStableAndOmitsValues(t *testing.T) {
	got := changedColumns(map[string]any{
		"password": "encrypted-blob",
		"host":     "smtp.example.com",
		"port":     587,
	})
	want := []string{"host", "port", "password"}
	if len(got) != len(want) {
		t.Fatalf("changedColumns = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("changedColumns = %v, want %v", got, want)
		}
	}
	for _, c := range got {
		if c == "encrypted-blob" {
			t.Fatal("changedColumns must report column names, never values")
		}
	}
}
