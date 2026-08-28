// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jkaninda/okapi"
)

func TestOkapiAutoRegistersOptionsPerPath(t *testing.T) {
	app := okapi.New()
	app.WithCORS(okapi.Cors{AllowedOrigins: []string{"https://dashboard.test"}})

	group := app.Group("/api/v1/f")
	app.Register(okapi.RouteDefinition{
		Method:  http.MethodPost,
		Path:    "/{key}",
		Handler: func(c *okapi.Context) error { return c.JSON(http.StatusAccepted, okapi.M{"ok": true}) },
		Group:   group,
	})

	panicked := func() (p bool) {
		defer func() {
			if recover() != nil {
				p = true
			}
		}()
		app.Register(okapi.RouteDefinition{
			Method:  http.MethodOptions,
			Path:    "/{key}",
			Handler: func(c *okapi.Context) error { return nil },
			Group:   group,
		})
		return false
	}()

	if !panicked {
		t.Fatal("okapi no longer auto-registers OPTIONS per path; the ingest route may now own its own preflight handler")
	}
}

func TestFormIngestPostIsNotAPreflightedRequest(t *testing.T) {
	app := okapi.New()
	app.WithCORS(okapi.Cors{AllowedOrigins: []string{"https://dashboard.test"}})

	group := app.Group("/api/v1/f")
	app.Register(okapi.RouteDefinition{
		Method: http.MethodPost,
		Path:   "/{key}",
		Handler: func(c *okapi.Context) error {
			c.ResponseWriter().Header().Set("Access-Control-Allow-Origin", c.Request().Header.Get("Origin"))
			return c.JSON(http.StatusAccepted, okapi.M{"ok": true})
		},
		Group: group,
	})

	for _, contentType := range []string{
		"text/plain;charset=UTF-8",
		"application/x-www-form-urlencoded",
		"multipart/form-data; boundary=x",
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/f/abc", nil)
		req.Header.Set("Origin", "https://customer.test")
		req.Header.Set("Content-Type", contentType)
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, req)

		if rec.Code != http.StatusAccepted {
			t.Fatalf("content type %q: status = %d, want 202", contentType, rec.Code)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://customer.test" {
			t.Fatalf("content type %q: allow-origin = %q", contentType, got)
		}
	}
}
