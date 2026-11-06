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
	"github.com/goposta/posta/internal/models"
	"github.com/jkaninda/okapi"
)

// Platform admins only, and a Posta Enterprise license is required.
func (r *Router) ldapRoutes() []okapi.RouteDefinition {
	admin := r.v1.Group("/admin/ldap", r.mw.jwtAdminAuth.Middleware).WithTagInfo(okapi.GroupTag{
		Name:        tagAdmin,
		Description: descAdmin,
	})
	admin.WithBearerAuth()
	id := okapi.DocPathParam("id", "integer", "Directory ID")

	return []okapi.RouteDefinition{
		{
			Method:      http.MethodGet,
			Path:        "",
			Handler:     r.h.ldapAdmin.List,
			Group:       admin,
			Middlewares: r.licensed(),
			Summary:     "List LDAP directories",
			Description: "The LDAP / Active Directory directories users sign in through, with their group mappings, in the order sign-in tries them.",
			Response:    &dto.Response[[]models.LDAPConfig]{},
		},
		{
			Method:      http.MethodPost,
			Path:        "",
			Handler:     okapi.H(r.h.ldapAdmin.Create),
			Group:       admin,
			Middlewares: r.licensedToChange(),
			Summary:     "Add an LDAP directory",
			Request:     &handlers.CreateLDAPConfigRequest{},
			Options:     []okapi.RouteOption{okapi.DocResponse(201, &dto.Response[models.LDAPConfig]{})},
		},
		{
			Method:      http.MethodPut,
			Path:        pathByID,
			Handler:     okapi.H(r.h.ldapAdmin.Update),
			Group:       admin,
			Middlewares: r.licensedToChange(),
			Summary:     "Update an LDAP directory",
			Description: "An empty bind_password keeps the stored one.",
			Request:     &handlers.UpdateLDAPConfigRequest{},
			Response:    &dto.Response[models.LDAPConfig]{},
			Options:     []okapi.RouteOption{id},
		},
		{
			Method:      http.MethodDelete,
			Path:        pathByID,
			Handler:     okapi.H(r.h.ldapAdmin.Delete),
			Group:       admin,
			Middlewares: r.licensedToChange(),
			Summary:     "Delete an LDAP directory",
			Description: "Accounts it created stay, but can no longer sign in until another directory or a password reset lets them.",
			Options:     []okapi.RouteOption{id, okapi.DocResponse(204, nil)},
		},
		{
			Method:      http.MethodPost,
			Path:        pathByID + "/test",
			Handler:     okapi.H(r.h.ldapAdmin.Test),
			Group:       admin,
			Middlewares: r.licensed(),
			Summary:     "Test an LDAP directory",
			Description: "Connects with the saved settings, binds as the service account and reads the user base DN. A failure is reported in the result, not as an error status.",
			Response:    &dto.Response[handlers.LDAPTestResponse]{},
			Options:     []okapi.RouteOption{id},
		},
		{
			Method:      http.MethodPost,
			Path:        pathByID + "/mappings",
			Handler:     okapi.H(r.h.ldapAdmin.CreateMapping),
			Group:       admin,
			Middlewares: r.licensedToChange(),
			Summary:     "Map a directory group to Posta access",
			Description: "Members of the group become platform admins, or join a workspace with a role. Applied at every sign-in, to accounts the directory created.",
			Request:     &handlers.CreateLDAPMappingRequest{},
			Options:     []okapi.RouteOption{id, okapi.DocResponse(201, &dto.Response[models.LDAPGroupMapping]{})},
		},
		{
			Method:      http.MethodDelete,
			Path:        pathByID + "/mappings/{mapping_id}",
			Handler:     okapi.H(r.h.ldapAdmin.DeleteMapping),
			Group:       admin,
			Middlewares: r.licensedToChange(),
			Summary:     "Remove a group mapping",
			Options: []okapi.RouteOption{
				id,
				okapi.DocPathParam("mapping_id", "integer", "Mapping ID"),
				okapi.DocResponse(204, nil),
			},
		},
	}
}
