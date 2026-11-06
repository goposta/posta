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
	"strings"
	"testing"

	"github.com/goposta/posta/internal/handlers"
	"github.com/goposta/posta/internal/services/tracking"
	"github.com/jkaninda/okapi"
)

func TestOpenPixelServesTheURLsTrackingBuilds(t *testing.T) {
	svc := tracking.NewService(nil, "https://posta.example.com", []byte("test-key"))
	app := okapi.New()
	r := &Router{app: app, h: routerHandlers{
		tracking: handlers.NewTrackingHandler(nil, nil, nil, nil, nil, nil, nil, svc, nil),
	}}
	app.Register(r.trackingRoutes()...)

	pixel := strings.TrimPrefix(svc.OpenURL(42), "https://posta.example.com")
	cases := map[string]int{
		pixel:                                 http.StatusOK,
		strings.Replace(pixel, "42", "43", 1): http.StatusNotFound,
		"/t/o/abc.gif?sig=x":                  http.StatusNotFound,
	}
	for path, want := range cases {
		w := httptest.NewRecorder()
		app.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != want {
			t.Errorf("GET %s = %d, want %d", path, w.Code, want)
		}
		if want == http.StatusOK && w.Header().Get("Content-Type") != "image/gif" {
			t.Errorf("GET %s Content-Type = %q, want image/gif", path, w.Header().Get("Content-Type"))
		}
	}
}
