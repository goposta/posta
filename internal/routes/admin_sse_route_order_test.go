// SPDX-FileCopyrightText: 2026 Jonas Kaninda
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
	o.Get("/admin/events/{id:int}", func(c *okapi.Context) error {
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
