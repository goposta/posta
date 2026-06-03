/*
 * Copyright 2026 Jonas Kaninda
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

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
