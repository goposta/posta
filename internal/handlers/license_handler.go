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

package handlers

import (
	"errors"
	"strings"
	"time"

	"github.com/goposta/posta/internal/enterprise"
	"github.com/goposta/posta/internal/services/audit"
	"github.com/jkaninda/okapi"
)

// expiryWarning is how long before expiry the dashboard starts warning.
const expiryWarning = 30 * 24 * time.Hour

type LicenseHandler struct {
	ee        enterprise.EE
	installID string
	audit     *audit.Logger
}

func NewLicenseHandler(ee enterprise.EE, installID string, auditLogger *audit.Logger) *LicenseHandler {
	return &LicenseHandler{ee: ee, installID: installID, audit: auditLogger}
}

type LicenseView struct {
	enterprise.Status
	// Given when buying a license; Status.InstallID is the one a license is bound to.
	InstanceInstallID string `json:"instance_install_id"`
	// One of license_grace, license_expired, nearing_expiry, binding_mismatch, unknown_signing_key.
	Warnings []string `json:"warnings"`
}

// Entitlements is what any signed-in user may know: enough for the dashboard to lock Enterprise features.
type Entitlements struct {
	Edition  string `json:"edition"`
	State    string `json:"state"`
	Licensed bool   `json:"licensed"`
	Mutable  bool   `json:"mutable"`
}

type InstallLicenseRequest struct {
	Body struct {
		Token string `json:"token" required:"true" description:"The license token: posta-v1.…"`
	} `json:"body"`
}

func (h *LicenseHandler) view() LicenseView {
	s := h.ee.Status()
	return LicenseView{Status: s, InstanceInstallID: h.installID, Warnings: licenseWarnings(s, time.Now())}
}

func licenseWarnings(s enterprise.Status, now time.Time) []string {
	out := []string{}
	switch s.State {
	case enterprise.StateGrace:
		out = append(out, "license_grace")
	case enterprise.StateDegraded:
		out = append(out, "license_expired")
	case enterprise.StateBindingMismatch, enterprise.StateUnknownKey:
		out = append(out, s.State)
	case enterprise.StateValid:
		if s.NotAfter != nil && s.NotAfter.Sub(now) < expiryWarning {
			out = append(out, "nearing_expiry")
		}
	}
	return out
}

func (h *LicenseHandler) Get(c *okapi.Context) error {
	return ok(c, h.view())
}

func (h *LicenseHandler) Entitlements(c *okapi.Context) error {
	s := h.ee.Status()
	return ok(c, Entitlements{Edition: s.Edition, State: s.State, Licensed: s.Licensed(), Mutable: s.Mutable()})
}

// Install verifies and stores a license. It takes effect immediately.
func (h *LicenseHandler) Install(c *okapi.Context, req *InstallLicenseRequest) error {
	token := strings.TrimSpace(req.Body.Token)
	if token == "" {
		return c.AbortBadRequest("license token is required")
	}
	s, err := h.ee.Install(c.Request().Context(), token, requestHost(c))
	if err != nil {
		if resp := licenseAbort(c, err); resp != nil {
			return resp
		}
		return c.AbortBadRequest(err.Error())
	}
	h.audit.LogCtx(c, "license.installed", "Posta Enterprise license installed", map[string]any{
		"license_id": s.LicenseID, "customer": s.Customer, "not_after": s.NotAfter,
	})
	return h.Get(c)
}

// Remove deletes the license. Enterprise features stop; their data stays.
func (h *LicenseHandler) Remove(c *okapi.Context) error {
	prev := h.ee.Status()
	if err := h.ee.Remove(c.Request().Context()); err != nil {
		if resp := licenseAbort(c, err); resp != nil {
			return resp
		}
		return c.AbortInternalServerError("failed to remove the license", err)
	}
	h.audit.LogCtx(c, "license.removed", "Posta Enterprise license removed", map[string]any{
		"license_id": prev.LicenseID, "customer": prev.Customer,
	})
	return h.Get(c)
}

// requestHost is the host the dashboard was reached on, behind a proxy too.
func requestHost(c *okapi.Context) string {
	r := c.Request()
	host := r.Host
	if fwd := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); fwd != "" {
		host = fwd
	}
	if i := strings.IndexByte(host, ','); i >= 0 {
		host = strings.TrimSpace(host[:i])
	}
	return host
}

// licenseAbort answers a license gate error with its own status and code (402 LICENSE_REQUIRED, ...), else nil.
func licenseAbort(c *okapi.Context, err error) error {
	var g enterprise.GateError
	if errors.As(err, &g) {
		return c.AbortWithError(g.HTTPStatus(), g)
	}
	return nil
}

func RequireLicense(ee enterprise.EE) okapi.Middleware {
	return func(c *okapi.Context) error {
		if err := ee.Require(); err != nil {
			return licenseAbort(c, err)
		}
		return c.Next()
	}
}

// RequireLicenseToChange also refuses once the license expired past grace, when Enterprise settings are read-only.
func RequireLicenseToChange(ee enterprise.EE) okapi.Middleware {
	return func(c *okapi.Context) error {
		if err := ee.RequireMutable(); err != nil {
			return licenseAbort(c, err)
		}
		return c.Next()
	}
}
