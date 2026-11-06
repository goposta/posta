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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jkaninda/okapi"
)

// A field named "body" makes okapi treat it as a body envelope and decode nothing, so bind through a real request.
func TestCreateAnnouncementRequestBinds(t *testing.T) {
	app := okapi.New()

	var got CreateAnnouncementRequest
	app.Register(okapi.RouteDefinition{
		Method: http.MethodPost,
		Path:   "/announcements",
		Handler: okapi.H(func(c *okapi.Context, req *CreateAnnouncementRequest) error {
			got = *req
			return c.JSON(http.StatusOK, okapi.M{"ok": true})
		}),
	})

	payload := `{"title":"Scheduled maintenance","message":"Posta is down 02:00-03:00 UTC.","link":"/status","severity":"warning"}`
	req := httptest.NewRequest(http.MethodPost, "/announcements", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if got.Body.Title != "Scheduled maintenance" {
		t.Errorf("title did not bind: %q", got.Body.Title)
	}
	if got.Body.Message != "Posta is down 02:00-03:00 UTC." {
		t.Errorf("message did not bind: %q", got.Body.Message)
	}
	if got.Body.Link != "/status" {
		t.Errorf("link did not bind: %q", got.Body.Link)
	}
	if got.Body.Severity != "warning" {
		t.Errorf("severity did not bind: %q", got.Body.Severity)
	}
}
