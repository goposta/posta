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
	"net/http/httptest"
	"testing"

	"github.com/jkaninda/okapi"
)

func TestAdminEventsStreamNotShadowedByParamRoute(t *testing.T) {
	o := okapi.NewTestServer(t)

	o.Get("/admin/events/stream", func(c *okapi.Context) error {
		return c.String(http.StatusOK, "stream")
	})
	o.Get("/admin/events/{id}", func(c *okapi.Context) error {
		return c.String(http.StatusOK, "detail")
	})

	cases := map[string]string{
		"/admin/events/stream": "stream",
		"/admin/events/42":     "detail",
	}
	for path, want := range cases {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		o.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, want 200", path, rec.Code)
		}
		if got := rec.Body.String(); got != want {
			t.Errorf("%s: handler = %q, want %q (route shadowing regression)", path, got, want)
		}
	}
}
