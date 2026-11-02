// Copyright (c) 2026 Jonas Kaninda. All rights reserved.
//
// This file is part of Posta Enterprise Edition.
//
// This file is licensed under the Posta Enterprise License, not the GNU
// Affero General Public License. See internal/enterprise/LICENSE.md.
//
// SPDX-License-Identifier: LicenseRef-Posta-Enterprise

package enterprise

import (
	"context"
	"errors"
	"testing"
)

func TestGates(t *testing.T) {
	cases := []struct {
		state            string
		licensed         bool
		require, mutable error
	}{
		{StateNone, false, ErrLicenseRequired, ErrLicenseRequired},
		{StateValid, true, nil, nil},
		{StateGrace, true, nil, nil},
		{StateDegraded, true, nil, ErrLicenseExpired},
		{StateBindingMismatch, false, ErrLicenseBindingMismatch, ErrLicenseBindingMismatch},
		{StateUnknownKey, false, ErrUnknownSigningKey, ErrUnknownSigningKey},
	}
	for _, tc := range cases {
		s := Status{State: tc.state}
		if s.Licensed() != tc.licensed {
			t.Errorf("%s: Licensed = %v", tc.state, s.Licensed())
		}
		if err := gate(s); !errors.Is(err, tc.require) {
			t.Errorf("%s: gate = %v, want %v", tc.state, err, tc.require)
		}
		if err := gateMutable(s); !errors.Is(err, tc.mutable) {
			t.Errorf("%s: gateMutable = %v, want %v", tc.state, err, tc.mutable)
		}
	}
}

func TestCommunity(t *testing.T) {
	ee := Community()
	if ee.Licensed() || ee.Mutable() || ee.Status().Build != EditionCommunity {
		t.Fatalf("got %+v", ee.Status())
	}
	if _, err := ee.Install(context.Background(), "posta-v1.k.e30.c2ln", ""); !errors.Is(err, ErrCommunityEdition) {
		t.Fatalf("install: %v", err)
	}
	var g GateError
	if !errors.As(ee.Require(), &g) || g.Code() != "LICENSE_REQUIRED" || g.HTTPStatus() != 402 {
		t.Fatalf("require: %v", ee.Require())
	}
}
