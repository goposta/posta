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
	"os"
	"testing"
)

// clearRedisEnv unsets every POSTA_REDIS_* var (Setenv registers restore,
// Unsetenv removes it so goutils.Env falls back to its default).
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

// TestNewRedisConfigBackwardCompatible: legacy addr/password (or nothing) keeps
// the same connection settings as before DB/URL support was added.
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

// TestNewRedisConfigDB verifies the new POSTA_REDIS_DB selector.
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

// TestNewRedisConfigURL: POSTA_REDIS_URL is parsed, overrides the discrete
// fields, and flows through both option builders.
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
	// Plain redis:// URL must not carry TLS.
	if rc.TLSConfig != nil {
		t.Fatalf("TLSConfig should be nil for redis:// URL")
	}
}

// TestNewRedisConfigTLS: a rediss:// URL enables TLS in both option builders.
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
