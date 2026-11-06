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

package routes

import (
	"net/http"

	"github.com/goposta/posta/internal/dto"
	"github.com/goposta/posta/internal/handlers"
	"github.com/jkaninda/okapi"
)

// Installing or removing a license takes an admin session, never an API key; any signed-in caller may read status.
func (r *Router) licenseRoutes() []okapi.RouteDefinition {
	admin := r.v1.Group("/admin/license", r.mw.jwtAdminAuth.Middleware).WithTagInfo(okapi.GroupTag{
		Name:        tagAdmin,
		Description: descAdmin,
	})
	admin.WithBearerAuth()

	return []okapi.RouteDefinition{
		{
			Method:      http.MethodGet,
			Path:        "",
			Handler:     r.h.license.Get,
			Group:       admin,
			Summary:     "Get the Enterprise license",
			Description: "The installed Posta Enterprise license, its state (none, valid, grace, degraded, binding_mismatch, unknown_signing_key), this installation's Install ID and any warnings.",
			Response:    &dto.Response[handlers.LicenseView]{},
		},
		{
			Method:      http.MethodPost,
			Path:        "",
			Handler:     okapi.H(r.h.license.Install),
			Group:       admin,
			Summary:     "Install an Enterprise license",
			Description: "Verifies a license token and installs it, replacing any other. Enterprise features unlock at once, with no restart. A license bound to another installation is refused with 402 LICENSE_BINDING_MISMATCH.",
			Request:     &handlers.InstallLicenseRequest{},
			Response:    &dto.Response[handlers.LicenseView]{},
			Options: []okapi.RouteOption{
				okapi.DocResponse(400, &dto.ErrorResponseBody{}),
				okapi.DocResponse(402, &dto.ErrorResponseBody{}),
			},
		},
		{
			Method:      http.MethodDelete,
			Path:        "",
			Handler:     r.h.license.Remove,
			Group:       admin,
			Summary:     "Remove the Enterprise license",
			Description: "Reverts to Community. Enterprise features stop; their data and settings are kept.",
			Response:    &dto.Response[handlers.LicenseView]{},
		},
		{
			Method:      http.MethodGet,
			Path:        "/system/license/entitlements",
			Handler:     r.h.license.Entitlements,
			Group:       r.v1,
			Middlewares: []okapi.Middleware{r.mw.auth},
			Tags:        []string{"Info"},
			Summary:     "Whether Enterprise features are on",
			Description: "licensed: Enterprise features run. mutable: their settings may change (false once a license has expired past its grace period).",
			Response:    &dto.Response[handlers.Entitlements]{},
		},
	}
}

func (r *Router) licensed() []okapi.Middleware {
	return []okapi.Middleware{handlers.RequireLicense(r.ee)}
}

// An expired license still runs Enterprise features but refuses settings changes with 402.
func (r *Router) licensedToChange() []okapi.Middleware {
	return []okapi.Middleware{handlers.RequireLicenseToChange(r.ee)}
}
