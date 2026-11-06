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
	"os"
	"slices"
	"testing"
)

// Unsetenv after Setenv keeps the restore but makes goutils.Env fall back to its default.
func clearRedisEnv(t *testing.T) {
	for _, k := range []string{
		"POSTA_REDIS_ADDR",
		"POSTA_REDIS_USERNAME",
		"POSTA_REDIS_PASSWORD",
		"POSTA_REDIS_DB",
		"POSTA_REDIS_URL",
	} {
		t.Setenv(k, "")
		_ = os.Unsetenv(k)
	}
}

// Legacy addr/password (or nothing) must keep the connection settings from before DB/URL support.
func TestNewRedisConfigBackwardCompatible(t *testing.T) {
	t.Run("defaults unchanged", func(t *testing.T) {
		clearRedisEnv(t)
		rc := newRedisConfig()
		if rc.Addr != "localhost:6379" {
			t.Fatalf("Addr = %q, want localhost:6379", rc.Addr)
		}
		if rc.Password != "" || rc.Username != "" {
			t.Fatalf("credentials = %q/%q, want empty", rc.Username, rc.Password)
		}
		if rc.DB != 0 {
			t.Fatalf("DB = %d, want 0", rc.DB)
		}
	})

	t.Run("legacy addr+password still honored", func(t *testing.T) {
		clearRedisEnv(t)
		t.Setenv("POSTA_REDIS_ADDR", "redis.internal:6380")
		t.Setenv("POSTA_REDIS_PASSWORD", "s3cret")
		rc := newRedisConfig()
		if rc.Addr != "redis.internal:6380" || rc.Password != "s3cret" {
			t.Fatalf("got %q/%q", rc.Addr, rc.Password)
		}
		if rc.DB != 0 {
			t.Fatalf("DB = %d, want 0", rc.DB)
		}
	})
}

func TestNewRedisConfigDB(t *testing.T) {
	clearRedisEnv(t)
	t.Setenv("POSTA_REDIS_ADDR", "localhost:6379")
	t.Setenv("POSTA_REDIS_DB", "3")
	rc := newRedisConfig()
	if rc.DB != 3 {
		t.Fatalf("DB = %d, want 3", rc.DB)
	}
	if got := rc.RedisOptions().DB; got != 3 {
		t.Fatalf("RedisOptions().DB = %d, want 3", got)
	}
	if got := rc.AsynqRedisOpt().DB; got != 3 {
		t.Fatalf("AsynqRedisOpt().DB = %d, want 3", got)
	}
}

func TestNewRedisConfigURL(t *testing.T) {
	clearRedisEnv(t)
	// Discrete values are set but must be overridden by the URL.
	t.Setenv("POSTA_REDIS_ADDR", "ignored:1111")
	t.Setenv("POSTA_REDIS_PASSWORD", "ignored")
	t.Setenv("POSTA_REDIS_URL", "redis://alice:hunter2@cache.example.com:6390/5")

	rc := newRedisConfig()
	if rc.Addr != "cache.example.com:6390" {
		t.Fatalf("Addr = %q", rc.Addr)
	}
	if rc.Username != "alice" || rc.Password != "hunter2" {
		t.Fatalf("creds = %q/%q", rc.Username, rc.Password)
	}
	if rc.DB != 5 {
		t.Fatalf("DB = %d, want 5", rc.DB)
	}

	ro := rc.RedisOptions()
	if ro.Addr != "cache.example.com:6390" || ro.Username != "alice" || ro.Password != "hunter2" || ro.DB != 5 {
		t.Fatalf("RedisOptions mismatch: %+v", ro)
	}
	ao := rc.AsynqRedisOpt()
	if ao.Addr != "cache.example.com:6390" || ao.Username != "alice" || ao.Password != "hunter2" || ao.DB != 5 {
		t.Fatalf("AsynqRedisOpt mismatch: %+v", ao)
	}
	if rc.TLSConfig != nil {
		t.Fatalf("TLSConfig should be nil for redis:// URL")
	}
}

func TestNewRedisConfigTLS(t *testing.T) {
	clearRedisEnv(t)
	t.Setenv("POSTA_REDIS_URL", "rediss://cache.example.com:6390/1")
	rc := newRedisConfig()
	if rc.TLSConfig == nil {
		t.Fatal("TLSConfig is nil, want TLS enabled for rediss://")
	}
	if rc.RedisOptions().TLSConfig == nil {
		t.Fatal("RedisOptions().TLSConfig is nil")
	}
	if rc.AsynqRedisOpt().TLSConfig == nil {
		t.Fatal("AsynqRedisOpt().TLSConfig is nil")
	}
}

func TestAllowedCORSOrigins(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  Config
		want []string
	}{
		{"explicit wins", Config{CORSOrigins: " https://a.test , https://b.test", AppWebURL: "https://web.test"}, []string{"https://a.test", "https://b.test"}},
		{"derived from urls", Config{AppWebURL: "https://web.test/app/", ApiBaseURL: "https://api.test:8443/v1"}, []string{"https://web.test", "https://api.test:8443"}},
		{"duplicate origin once", Config{AppWebURL: "https://posta.test", ApiBaseURL: "https://posta.test/api"}, []string{"https://posta.test"}},
		{"nothing configured", Config{}, nil},
		{"unparseable url skipped", Config{AppWebURL: "posta.test"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cfg.AllowedCORSOrigins(); !slices.Equal(got, tc.want) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
