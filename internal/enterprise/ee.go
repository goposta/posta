// Copyright (c) 2026 Jonas Kaninda. All rights reserved.
//
// This file is part of Posta Enterprise Edition.
//
// This file is licensed under the Posta Enterprise License, not the GNU
// Affero General Public License. See internal/enterprise/LICENSE.md.
//
// SPDX-License-Identifier: LicenseRef-Posta-Enterprise

// Package enterprise is the seam between Posta Community and Posta
// Enterprise. Core code asks one interface, EE, whether Enterprise features
// may run. The Community build links a stub that always says no; the
// Enterprise build (the `enterprise` tag) links the license-verifying
// implementation. The HTTP layer is the same in both, so a Community binary
// answers 402 and contains no verification code.
//
// Posta has one paid edition. A license is either there or not: there are
// no tiers and no per-feature flags, and a valid license unlocks every
// Enterprise feature of the running build.
package enterprise

import (
	"context"
	"time"
)

// Product is the product name license tokens are issued for.
const Product = "posta"

// Editions.
const (
	EditionCommunity  = "community"
	EditionEnterprise = "enterprise"
)

// States. A license moves from valid to grace to degraded as it expires.
const (
	StateNone            = "none"     // no license: Community
	StateValid           = "valid"    // within term
	StateGrace           = "grace"    // past expiry, within the grace days: works, with a warning
	StateDegraded        = "degraded" // past grace: features run, their settings are read-only
	StateBindingMismatch = "binding_mismatch"
	// StateUnknownKey is a stored license signed by a key this build does not
	// trust, for example after a downgrade. It grants nothing but is reported,
	// never mistaken for "no license".
	StateUnknownKey = "unknown_signing_key"
)

// Binding failures, reported in Status.BindingError.
const (
	BindingInstallID = "install_id"
	BindingURL       = "url"
)

// Status is the installed license as of now.
type Status struct {
	Edition string `json:"edition"`
	State   string `json:"state"`
	// Build is the edition this binary was built as. A Community build
	// cannot install a license at all.
	Build        string     `json:"build"`
	LicenseID    string     `json:"license_id,omitempty"`
	Customer     string     `json:"customer,omitempty"`
	Trial        bool       `json:"trial,omitempty"`
	InstallID    string     `json:"install_id,omitempty"` // the Install ID the license is bound to
	URL          string     `json:"url,omitempty"`        // the host the license is bound to
	BindingError string     `json:"binding_error,omitempty"`
	NotAfter     *time.Time `json:"not_after,omitempty"`
	GraceEnds    *time.Time `json:"grace_ends,omitempty"`
}

// Licensed reports whether Enterprise features run in this state.
func (s Status) Licensed() bool {
	return s.State == StateValid || s.State == StateGrace || s.State == StateDegraded
}

// Mutable reports whether Enterprise settings may change in this state.
func (s Status) Mutable() bool {
	return s.State == StateValid || s.State == StateGrace
}

// EE gates Enterprise features. Every method is safe on the Community stub.
//
// Use Licensed for behaviour (does a routing rule apply?), Mutable or
// RequireMutable before changing Enterprise settings, and Require before
// serving an Enterprise endpoint that changes nothing. State is evaluated on
// every call, so a license that expires or is renewed needs no restart.
type EE interface {
	Status() Status
	Licensed() bool
	Mutable() bool
	// Require returns nil when Enterprise features may run, otherwise the
	// gate error explaining why not.
	Require() error
	// RequireMutable is Require plus ErrLicenseExpired once the license is
	// degraded: an expired license keeps Enterprise features running but
	// freezes their settings until it is renewed.
	RequireMutable() error
	// Install verifies a license token and stores it, replacing any other.
	// requestHost is the host the admin installs from; a license bound to a
	// URL must match it or the configured web URL.
	Install(ctx context.Context, token, requestHost string) (Status, error)
	// Remove deletes the installed license, reverting to Community.
	Remove(ctx context.Context) error
}

// Options configure New.
type Options struct {
	// License is a license token to install at boot when none is stored
	// (POSTA_LICENSE), and LicenseFile a file holding one
	// (POSTA_LICENSE_FILE). A stored license wins over both, so a license
	// pasted in the dashboard is not reverted by a restart.
	License     string
	LicenseFile string
	// InstanceURL is this deployment's public web URL, for the URL binding.
	InstanceURL string
	// InstallID is this deployment's Install ID, for the install-ID binding.
	InstallID string
}

// GateError is the error a gate returns. Code is the stable API error code
// and HTTPStatus the response status.
type GateError interface {
	error
	Code() string
	HTTPStatus() int
}

type gateError struct {
	code   string
	msg    string
	status int
}

func (e *gateError) Error() string   { return e.msg }
func (e *gateError) Code() string    { return e.code }
func (e *gateError) HTTPStatus() int { return e.status }

// Gate errors. They are pointers, so errors.Is matches by identity.
var (
	ErrLicenseRequired        = &gateError{"LICENSE_REQUIRED", "this feature requires a Posta Enterprise license", 402}
	ErrLicenseExpired         = &gateError{"LICENSE_EXPIRED", "your Posta Enterprise license has expired; this feature is read-only until it is renewed", 402}
	ErrLicenseBindingMismatch = &gateError{"LICENSE_BINDING_MISMATCH", "this license is bound to a different Posta installation (Install ID or URL)", 402}
	ErrUnknownSigningKey      = &gateError{"LICENSE_UNKNOWN_KEY", "this license is signed by a key this version of Posta does not know; upgrade Posta or ask for a reissued license", 402}
	ErrCommunityEdition       = &gateError{"COMMUNITY_EDITION", "this Posta binary was built without Enterprise support, so it cannot install a license", 402}
)

// community is the answer when Enterprise features may not run: the
// Community build's only implementation, and the Enterprise build's state
// before a license is installed.
type community struct{}

func (community) Status() Status {
	return Status{Edition: EditionCommunity, State: StateNone, Build: EditionCommunity}
}
func (community) Licensed() bool               { return false }
func (community) Mutable() bool                { return false }
func (community) Require() error               { return ErrLicenseRequired }
func (community) RequireMutable() error        { return ErrLicenseRequired }
func (community) Remove(context.Context) error { return ErrCommunityEdition }
func (community) Install(context.Context, string, string) (Status, error) {
	return Status{}, ErrCommunityEdition
}

// Community returns the Community answer: no Enterprise features. It is the
// safe default wherever an EE has not been wired in.
func Community() EE { return community{} }

// gate turns a status into the error Require returns.
func gate(s Status) error {
	switch s.State {
	case StateValid, StateGrace, StateDegraded:
		return nil
	case StateBindingMismatch:
		return ErrLicenseBindingMismatch
	case StateUnknownKey:
		return ErrUnknownSigningKey
	}
	return ErrLicenseRequired
}

// gateMutable turns a status into the error RequireMutable returns.
func gateMutable(s Status) error {
	if err := gate(s); err != nil {
		return err
	}
	if !s.Mutable() {
		return ErrLicenseExpired
	}
	return nil
}
