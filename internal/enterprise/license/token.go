// Copyright (c) 2026 Jonas Kaninda. All rights reserved.
//
// This file is part of Posta Enterprise Edition.
//
// This file is licensed under the Posta Enterprise License, not the GNU
// Affero General Public License. See internal/enterprise/LICENSE.md.
//
// SPDX-License-Identifier: LicenseRef-Posta-Enterprise

// Package license only verifies <product>-v1.<kid>.<claims>.<ed25519 sig> tokens; the signature covers the kid.
package license

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const Version = "v1"

type State string

const (
	StateNone     State = "none"     // not yet active
	StateValid    State = "valid"    // within term
	StateGrace    State = "grace"    // past expiry, within the grace days
	StateDegraded State = "degraded" // past grace: features run, their settings are read-only
)

var (
	ErrMalformed    = errors.New("license: malformed token")
	ErrUnknownKey   = errors.New("license: signed by a key this build does not trust")
	ErrBadSignature = errors.New("license: signature verification failed")
	ErrWrongProduct = errors.New("license: issued for another product")
	ErrBadKeyring   = errors.New("license: invalid keyring")
)

type Claims struct {
	Product   string         `json:"product"`
	LicenseID string         `json:"license_id"`
	Customer  string         `json:"customer"`
	Edition   string         `json:"edition"`
	Tier      string         `json:"tier,omitempty"`
	InstallID string         `json:"install_id,omitempty"`
	URL       string         `json:"url,omitempty"`
	Flags     []string       `json:"flags,omitempty"`
	Limits    map[string]int `json:"limits,omitempty"`
	Trial     bool           `json:"trial,omitempty"`
	NotBefore time.Time      `json:"not_before"`
	NotAfter  time.Time      `json:"not_after"`
	GraceDays int            `json:"grace_days"`
	IssuedAt  time.Time      `json:"issued_at"`
}

type Keyring struct {
	Product string
	Keys    map[string]ed25519.PublicKey
}

type keyringJSON struct {
	Product string            `json:"product"`
	Legacy  string            `json:"legacy,omitempty"`
	Keys    map[string]string `json:"keys"`
}

var b64 = base64.RawURLEncoding

func ParseKeyring(raw string) (Keyring, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Keyring{}, ErrBadKeyring
	}
	data := []byte(raw)
	if !strings.HasPrefix(raw, "{") {
		decoded, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return Keyring{}, ErrBadKeyring
		}
		data = decoded
	}
	var kj keyringJSON
	if err := json.Unmarshal(data, &kj); err != nil {
		return Keyring{}, ErrBadKeyring
	}
	ring := Keyring{Product: kj.Product, Keys: map[string]ed25519.PublicKey{}}
	for kid, pub := range kj.Keys {
		key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(pub))
		if err != nil || len(key) != ed25519.PublicKeySize {
			return Keyring{}, fmt.Errorf("%w: key %q", ErrBadKeyring, kid)
		}
		ring.Keys[kid] = ed25519.PublicKey(key)
	}
	if ring.Product == "" || len(ring.Keys) == 0 {
		return Keyring{}, ErrBadKeyring
	}
	return ring, nil
}

func KeyID(token string) string {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 4 {
		return ""
	}
	return parts[1]
}

func Verify(ring Keyring, product, token string) (Claims, error) {
	var c Claims
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 4 || parts[0] != product+"-"+Version || parts[1] == "" {
		return c, ErrMalformed
	}
	if ring.Product != product {
		return c, ErrWrongProduct
	}
	key, ok := ring.Keys[parts[1]]
	if !ok {
		return c, ErrUnknownKey
	}
	payload, err := b64.DecodeString(parts[2])
	if err != nil {
		return c, ErrMalformed
	}
	sig, err := b64.DecodeString(parts[3])
	if err != nil {
		return c, ErrMalformed
	}
	if !ed25519.Verify(key, []byte(parts[0]+"."+parts[1]+"."+parts[2]), sig) {
		return c, ErrBadSignature
	}
	if err := json.Unmarshal(payload, &c); err != nil {
		return c, fmt.Errorf("%w: bad claims", ErrMalformed)
	}
	if c.Product != product {
		return c, ErrWrongProduct
	}
	return c, nil
}

func Evaluate(c Claims, now time.Time) (State, time.Time) {
	graceEnds := c.NotAfter.AddDate(0, 0, c.GraceDays)
	switch {
	case !c.NotBefore.IsZero() && now.Before(c.NotBefore):
		return StateNone, graceEnds
	case now.Before(c.NotAfter):
		return StateValid, graceEnds
	case now.Before(graceEnds):
		return StateGrace, graceEnds
	default:
		return StateDegraded, graceEnds
	}
}
