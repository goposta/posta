// Copyright (c) 2026 Jonas Kaninda. All rights reserved.
//
// This file is part of Posta Enterprise Edition.
//
// This file is licensed under the Posta Enterprise License, not the GNU
// Affero General Public License. See internal/enterprise/LICENSE.md.
//
// SPDX-License-Identifier: LicenseRef-Posta-Enterprise

package license

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func sign(t *testing.T, product, kid string, priv ed25519.PrivateKey, c Claims) string {
	t.Helper()
	payload, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	body := product + "-v1." + kid + "." + b64.EncodeToString(payload)
	return body + "." + b64.EncodeToString(ed25519.Sign(priv, []byte(body)))
}

func newKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

func ringJSON(product string, keys map[string]ed25519.PublicKey) string {
	enc := map[string]string{}
	for kid, k := range keys {
		enc[kid] = base64.StdEncoding.EncodeToString(k)
	}
	raw, _ := json.Marshal(map[string]any{"product": product, "keys": enc})
	return string(raw)
}

func TestVerify(t *testing.T) {
	pub, priv := newKey(t)
	standby, standbyPriv := newKey(t)
	ring, err := ParseKeyring(ringJSON("posta", map[string]ed25519.PublicKey{"posta-202611-a": pub, "posta-202611-b": standby}))
	if err != nil {
		t.Fatal(err)
	}
	claims := Claims{Product: "posta", LicenseID: "lic_1", Customer: "Acme", Edition: "enterprise",
		NotAfter: time.Now().AddDate(1, 0, 0), GraceDays: 14}

	t.Run("valid with the active and the standby key", func(t *testing.T) {
		for kid, key := range map[string]ed25519.PrivateKey{"posta-202611-a": priv, "posta-202611-b": standbyPriv} {
			got, err := Verify(ring, "posta", sign(t, "posta", kid, key, claims))
			if err != nil || got.Customer != "Acme" {
				t.Fatalf("%s: got %+v, %v", kid, got, err)
			}
		}
	})

	_, otherPriv := newKey(t)
	cases := []struct {
		name  string
		token string
		want  error
	}{
		{"unknown kid", sign(t, "posta", "posta-209901-x", otherPriv, claims), ErrUnknownKey},
		{"wrong key under a known kid", sign(t, "posta", "posta-202611-a", otherPriv, claims), ErrBadSignature},
		{"another product's prefix", sign(t, "miabi", "posta-202611-a", priv, claims), ErrMalformed},
		{"another product's claim", sign(t, "posta", "posta-202611-a", priv, Claims{Product: "stora", NotAfter: claims.NotAfter}), ErrWrongProduct},
		{"v1 token", "posta-v1.e30.c2ln", ErrMalformed},
		{"garbage", "not a token", ErrMalformed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Verify(ring, "posta", tc.token); !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}

	t.Run("swapped kid breaks the signature", func(t *testing.T) {
		token := sign(t, "posta", "posta-202611-a", priv, claims)
		swapped := "posta-v1.posta-202611-b" + token[len("posta-v1.posta-202611-a"):]
		if _, err := Verify(ring, "posta", swapped); !errors.Is(err, ErrBadSignature) {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("edited claims break the signature", func(t *testing.T) {
		parts := strings.Split(sign(t, "posta", "posta-202611-a", priv, claims), ".")
		forged, _ := json.Marshal(Claims{Product: "posta", Customer: "Acme", NotAfter: time.Now().AddDate(50, 0, 0)})
		edited := fmt.Sprintf("posta-v1.posta-202611-a.%s.%s", b64.EncodeToString(forged), parts[3])
		if _, err := Verify(ring, "posta", edited); !errors.Is(err, ErrBadSignature) {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("a keyring for another product", func(t *testing.T) {
		miabi, _ := ParseKeyring(ringJSON("miabi", map[string]ed25519.PublicKey{"posta-202611-a": pub}))
		if _, err := Verify(miabi, "posta", sign(t, "posta", "posta-202611-a", priv, claims)); !errors.Is(err, ErrWrongProduct) {
			t.Fatalf("got %v", err)
		}
	})
}

func TestParseKeyring(t *testing.T) {
	pub, _ := newKey(t)
	raw := ringJSON("posta", map[string]ed25519.PublicKey{"k1": pub})
	for name, in := range map[string]string{"json": raw, "base64": base64.StdEncoding.EncodeToString([]byte(raw))} {
		ring, err := ParseKeyring(in)
		if err != nil || len(ring.Keys) != 1 || ring.Product != "posta" {
			t.Fatalf("%s: %+v, %v", name, ring, err)
		}
	}
	for _, bad := range []string{"", "{}", `{"product":"posta","keys":{"k1":"c2hvcnQ="}}`, "%%%"} {
		if _, err := ParseKeyring(bad); !errors.Is(err, ErrBadKeyring) {
			t.Fatalf("%q: got %v", bad, err)
		}
	}
}

func TestEvaluate(t *testing.T) {
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	c := Claims{NotAfter: now.AddDate(0, 0, 10), GraceDays: 14}
	cases := []struct {
		at   time.Time
		want State
	}{
		{now, StateValid},
		{now.AddDate(0, 0, 11), StateGrace},
		{now.AddDate(0, 0, 25), StateDegraded},
	}
	for _, tc := range cases {
		if got, _ := Evaluate(c, tc.at); got != tc.want {
			t.Fatalf("at %s: got %s, want %s", tc.at, got, tc.want)
		}
	}
	c.NotBefore = now.AddDate(0, 0, 1)
	if got, _ := Evaluate(c, now); got != StateNone {
		t.Fatalf("before not_before: got %s", got)
	}
}
