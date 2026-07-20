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

package session

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jkaninda/okapi"
)

// A JWT written by SetCookie must come back byte-identical from Cookie(),
// otherwise okapi's cookie TokenLookup hands a corrupted token to the parser.
func TestSetCookie_RoundTripsJWT(t *testing.T) {
	const jwt = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOjEsImp0aSI6ImEtYl9jIn0.7Hy-_signature"

	o := okapi.NewTestServer(t)
	o.Get("/login", func(c *okapi.Context) error {
		SetCookie(c, jwt, time.Hour)
		return c.String(http.StatusOK, "ok")
	})
	o.Get("/echo", func(c *okapi.Context) error {
		v, err := c.Cookie(CookieName)
		if err != nil {
			return c.String(http.StatusUnauthorized, "missing")
		}
		return c.String(http.StatusOK, v)
	})

	rec := httptest.NewRecorder()
	o.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/login", nil))

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies, want 1", len(cookies))
	}
	ck := cookies[0]

	if ck.Name != CookieName {
		t.Errorf("name = %q, want %q", ck.Name, CookieName)
	}
	if !ck.HttpOnly {
		t.Error("cookie must be HttpOnly so XSS cannot read the session")
	}
	if ck.SameSite != http.SameSiteStrictMode {
		t.Errorf("SameSite = %v, want Strict", ck.SameSite)
	}
	if ck.Path != "/" {
		t.Errorf("path = %q, want /", ck.Path)
	}
	if ck.Secure {
		t.Error("plain-HTTP request must not get a Secure cookie, the browser would drop it")
	}

	echo := httptest.NewRequest(http.MethodGet, "/echo", nil)
	echo.AddCookie(ck)
	rec2 := httptest.NewRecorder()
	o.ServeHTTP(rec2, echo)

	if got := rec2.Body.String(); got != jwt {
		t.Errorf("round-tripped token = %q, want %q", got, jwt)
	}
}

func TestSetCookie_SecureBehindTLSTerminatingProxy(t *testing.T) {
	o := okapi.NewTestServer(t)
	o.Get("/login", func(c *okapi.Context) error {
		SetCookie(c, "tok", time.Hour)
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	o.ServeHTTP(rec, req)

	if ck := rec.Result().Cookies()[0]; !ck.Secure {
		t.Error("cookie must be Secure when the edge terminated TLS")
	}
}

// ClearCookie must expire the cookie; its other attributes have to match
// SetCookie's or the browser keeps the original.
func TestClearCookie_Expires(t *testing.T) {
	o := okapi.NewTestServer(t)
	o.Get("/logout", func(c *okapi.Context) error {
		ClearCookie(c)
		return c.String(http.StatusOK, "ok")
	})

	rec := httptest.NewRecorder()
	o.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/logout", nil))

	ck := rec.Result().Cookies()[0]
	if ck.Name != CookieName || ck.Value != "" {
		t.Errorf("cookie = %s=%q, want %s empty", ck.Name, ck.Value, CookieName)
	}
	if ck.MaxAge >= 0 {
		t.Errorf("MaxAge = %d, want negative", ck.MaxAge)
	}
	if ck.Path != "/" || !ck.HttpOnly || ck.SameSite != http.SameSiteStrictMode {
		t.Error("clearing cookie attributes must match the ones SetCookie wrote")
	}
}
