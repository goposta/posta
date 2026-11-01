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
	"testing"

	"github.com/goposta/posta/internal/config"
	"github.com/jkaninda/okapi"
)

// The server validates the OpenAPI document at startup and only logs a
// warning, so a broken document (two operations with one operationId, an
// undeclared security scheme) would otherwise ship unnoticed. okapi derives
// an operationId from the summary, so two routes with the same summary clash.
func TestOpenAPIDocumentIsValid(t *testing.T) {
	app := okapi.New().WithOpenAPIDocs(okapi.OpenAPI{
		Title:           "Posta API",
		Version:         "test",
		SecuritySchemes: config.OpenAPISecuritySchemes(),
	})
	r := &Router{
		app: app,
		cfg: &config.Config{InboundEnabled: true, MessagesEnabled: true},
		v1:  app.Group("/api/v1"),
	}
	app.Register(r.healthRoutes()...)
	app.Register(r.infoRoute())
	app.Register(r.authRoutes()...)
	app.Register(r.apiAuthRoutes()...)
	app.Register(r.userRoutes()...)
	app.Register(r.workspaceRoutes()...)
	app.Register(r.workspaceResourceRoutes()...)
	app.Register(r.oauthRoutes()...)
	app.Register(r.trackingRoutes()...)
	app.Register(r.trackingAnalyticsRoutes()...)
	app.Register(r.bounceWebhookRoutes()...)
	app.Register(r.inboundWebhookRoutes()...)
	app.Register(r.inboundWorkspaceRoutes()...)
	app.Register(r.formIngestRoutes()...)
	app.Register(r.messageWorkspaceRoutes()...)
	app.Register(r.adminSSERoutes()...)
	app.Register(r.adminRoutes()...)

	if err := app.ValidateOpenAPI(); err != nil {
		t.Fatal(err)
	}
}
