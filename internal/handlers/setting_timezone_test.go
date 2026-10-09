// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jkaninda/okapi"
)

// Both settings endpoints accept only IANA names; an omitted timezone leaves
// the stored one alone.
func TestSettingsTimezoneValidation(t *testing.T) {
	app := okapi.New()
	ok := func(c *okapi.Context) error { return c.String(http.StatusOK, "ok") }
	app.Put("/workspace", okapi.H(func(c *okapi.Context, _ *UpdateWorkspaceSettingsRequest) error { return ok(c) }))
	app.Put("/user", okapi.H(func(c *okapi.Context, _ *UpdateUserSettingsRequest) error { return ok(c) }))

	cases := []struct {
		body string
		want int
	}{
		{`{}`, http.StatusOK},
		{`{"timezone":"UTC"}`, http.StatusOK},
		{`{"timezone":"Africa/Kinshasa"}`, http.StatusOK},
		{`{"timezone":"America/Argentina/Buenos_Aires"}`, http.StatusOK},
		{`{"timezone":""}`, http.StatusBadRequest},
		{`{"timezone":"Local"}`, http.StatusBadRequest},
		{`{"timezone":"Mars/Olympus"}`, http.StatusBadRequest},
		{`{"timezone":"+02:00"}`, http.StatusBadRequest},
	}
	for _, path := range []string{"/workspace", "/user"} {
		for _, tc := range cases {
			req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			app.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Errorf("%s %s: status %d, want %d (%s)", path, tc.body, rec.Code, tc.want, rec.Body.String())
			}
		}
	}
}

// A subscriber may have no timezone, but a given one must be an IANA name.
func TestSubscriberTimezoneValidation(t *testing.T) {
	app := okapi.New()
	ok := func(c *okapi.Context) error { return c.String(http.StatusOK, "ok") }
	app.Post("/subscribers", okapi.H(func(c *okapi.Context, _ *CreateSubscriberRequest) error { return ok(c) }))
	app.Put("/subscribers/{id}", okapi.H(func(c *okapi.Context, _ *UpdateSubscriberRequest) error { return ok(c) }))

	for _, tc := range []struct {
		tz   string
		want int
	}{
		{`""`, http.StatusOK},
		{`"Africa/Kinshasa"`, http.StatusOK},
		{`"Mars/Olympus"`, http.StatusBadRequest},
		{`"Local"`, http.StatusBadRequest},
	} {
		for _, r := range []struct{ method, path string }{{http.MethodPost, "/subscribers"}, {http.MethodPut, "/subscribers/1"}} {
			body := `{"email":"a@example.com","timezone":` + tc.tz + `}`
			req := httptest.NewRequest(r.method, r.path, strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			app.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Errorf("%s %s timezone=%s: status %d, want %d (%s)", r.method, r.path, tc.tz, rec.Code, tc.want, rec.Body.String())
			}
		}
	}
}

// Imports and restores keep the subscriber and drop only a bad timezone.
func TestSubscriberTimezoneNormalisation(t *testing.T) {
	for _, tc := range []struct {
		in, want string
		valid    bool
	}{
		{"", "", true},
		{"  Europe/Paris ", "Europe/Paris", true},
		{"UTC", "UTC", true},
		{"Local", "", false},
		{"GMT+2", "", false},
		{"Mars/Olympus", "", false},
	} {
		got, valid := subscriberTimezone(tc.in)
		if got != tc.want || valid != tc.valid {
			t.Errorf("subscriberTimezone(%q) = %q, %v; want %q, %v", tc.in, got, valid, tc.want, tc.valid)
		}
	}
}
