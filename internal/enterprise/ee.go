// Copyright (c) 2026 Jonas Kaninda. All rights reserved.
//
// This file is part of Posta Enterprise Edition.
//
// This file is licensed under the Posta Enterprise License, not the GNU
// Affero General Public License. See internal/enterprise/LICENSE.md.
//
// SPDX-License-Identifier: LicenseRef-Posta-Enterprise

// Package enterprise gates EE features: Community links a stub answering 402; one paid edition, no per-feature tiers.
package enterprise

import (
	"context"
	"errors"
	"time"

	"github.com/goposta/posta/internal/models"
)

const Product = "posta"

const (
	EditionCommunity  = "community"
	EditionEnterprise = "enterprise"
)

// A license moves from valid to grace to degraded as it expires.
const (
	StateNone            = "none"     // no license: Community
	StateValid           = "valid"    // within term
	StateGrace           = "grace"    // past expiry, within the grace days: works, with a warning
	StateDegraded        = "degraded" // past grace: features run, their settings are read-only
	StateBindingMismatch = "binding_mismatch"
	// A license signed by a key this build does not trust grants nothing, but is never mistaken for "no license".
	StateUnknownKey = "unknown_signing_key"
)

const (
	BindingInstallID = "install_id"
	BindingURL       = "url"
)

type Status struct {
	Edition string `json:"edition"`
	State   string `json:"state"`
	// A Community build cannot install a license at all.
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

func (s Status) Licensed() bool {
	return s.State == StateValid || s.State == StateGrace || s.State == StateDegraded
}

func (s Status) Mutable() bool {
	return s.State == StateValid || s.State == StateGrace
}

// Licensed gates behaviour, (Require)Mutable guards settings changes; state is re-evaluated per call, no restart.
type EE interface {
	Status() Status
	Licensed() bool
	Mutable() bool
	Require() error
	// An expired (degraded) license keeps features running but freezes their settings until renewed.
	RequireMutable() error
	// A URL-bound license must match requestHost or the configured web URL.
	Install(ctx context.Context, token, requestHost string) (Status, error)
	Remove(ctx context.Context) error
	// Nil without a license; sign-in asks it only after the local password fails, so local accounts survive an outage.
	LDAP() LDAPAuthenticator
}

// Plain data, so this package never touches users or sessions.
type LDAPIdentity struct {
	Email     string
	Name      string
	Groups    []string // group DNs
	Directory string   // the matched LDAPConfig name
}

type LDAPTestResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

type LDAPAuthenticator interface {
	Authenticate(ctx context.Context, email, password string) (LDAPIdentity, error)
	TestConnection(ctx context.Context, cfg *models.LDAPConfig) LDAPTestResult
}

// ErrLDAPNoMatch keeps sign-in on its local result and is never shown to the client.
var ErrLDAPNoMatch = errors.New("no matching LDAP directory")

type Options struct {
	// A stored license wins over POSTA_LICENSE(_FILE), so one pasted in the dashboard survives a restart.
	License     string
	LicenseFile string
	InstanceURL string
	InstallID   string
	// Passed in, in sign-in order, so this package imports no repositories or crypto.
	LDAPDirectories func(ctx context.Context) ([]models.LDAPConfig, error)
	Decrypt         func(stored string) (string, error)
}

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

// Pointers, so errors.Is matches by identity.
var (
	ErrLicenseRequired        = &gateError{"LICENSE_REQUIRED", "this feature requires a Posta Enterprise license", 402}
	ErrLicenseExpired         = &gateError{"LICENSE_EXPIRED", "your Posta Enterprise license has expired; this feature is read-only until it is renewed", 402}
	ErrLicenseBindingMismatch = &gateError{"LICENSE_BINDING_MISMATCH", "this license is bound to a different Posta installation (Install ID or URL)", 402}
	ErrUnknownSigningKey      = &gateError{"LICENSE_UNKNOWN_KEY", "this license is signed by a key this version of Posta does not know; upgrade Posta or ask for a reissued license", 402}
	ErrCommunityEdition       = &gateError{"COMMUNITY_EDITION", "this Posta binary was built without Enterprise support, so it cannot install a license", 402}
)

// community is also the Enterprise build's state before a license is installed.
type community struct{}

func (community) Status() Status {
	return Status{Edition: EditionCommunity, State: StateNone, Build: EditionCommunity}
}
func (community) Licensed() bool               { return false }
func (community) Mutable() bool                { return false }
func (community) Require() error               { return ErrLicenseRequired }
func (community) RequireMutable() error        { return ErrLicenseRequired }
func (community) Remove(context.Context) error { return ErrCommunityEdition }
func (community) LDAP() LDAPAuthenticator      { return nil }
func (community) Install(context.Context, string, string) (Status, error) {
	return Status{}, ErrCommunityEdition
}

// Community is the safe default wherever an EE has not been wired in.
func Community() EE { return community{} }

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

func gateMutable(s Status) error {
	if err := gate(s); err != nil {
		return err
	}
	if !s.Mutable() {
		return ErrLicenseExpired
	}
	return nil
}
