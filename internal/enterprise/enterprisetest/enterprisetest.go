// Copyright (c) 2026 Jonas Kaninda. All rights reserved.
//
// This file is part of Posta Enterprise Edition.
//
// This file is licensed under the Posta Enterprise License, not the GNU
// Affero General Public License. See internal/enterprise/LICENSE.md.
//
// SPDX-License-Identifier: LicenseRef-Posta-Enterprise

// Package enterprisetest provides license states for tests of code that is
// gated on the Enterprise license, in either build.
package enterprisetest

import (
	"context"

	"github.com/goposta/posta/internal/enterprise"
)

// State is an EE fixed in one license state. It verifies nothing.
type State string

// Licensed is a valid license, Degraded one expired past its grace period,
// and Community no license.
const (
	Licensed  State = enterprise.StateValid
	Degraded  State = enterprise.StateDegraded
	Community State = enterprise.StateNone
)

func (s State) Status() enterprise.Status {
	st := enterprise.Status{Edition: enterprise.EditionEnterprise, State: string(s), Build: enterprise.EditionEnterprise}
	if s == Community {
		st.Edition = enterprise.EditionCommunity
	}
	return st
}

func (s State) Licensed() bool { return s.Status().Licensed() }
func (s State) Mutable() bool  { return s.Status().Mutable() }

func (s State) Require() error {
	if !s.Licensed() {
		return enterprise.ErrLicenseRequired
	}
	return nil
}

func (s State) RequireMutable() error {
	if err := s.Require(); err != nil {
		return err
	}
	if !s.Mutable() {
		return enterprise.ErrLicenseExpired
	}
	return nil
}

func (s State) Install(context.Context, string, string) (enterprise.Status, error) {
	return s.Status(), nil
}

func (s State) Remove(context.Context) error { return nil }
