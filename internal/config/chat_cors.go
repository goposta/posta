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

package config

import (
	"net/http"
	"strings"

	"github.com/jkaninda/okapi"
)

// ChatAPIPrefix is where the public live chat API lives.
const ChatAPIPrefix = "/api/v1/chat/"

// ChatPreflight answers CORS preflights for the public chat API, which
// websites Posta does not know call with an Authorization header.
//
// The global CORS handler would refuse them: it allows the dashboard's
// origins only, and it answers preflights before any route runs. Which
// origins may use a chat widget is the widget's own allowlist, and a
// preflight does not say which widget it is for in a way worth a database
// read, so this one allows any origin. That grants nothing: every real
// request checks the widget's allowlist before acting, answers a foreign
// origin without CORS headers, and the API uses no cookies, so the browser
// never sends credentials and Access-Control-Allow-Credentials is never set.
func ChatPreflight(c *okapi.Context) error {
	r := c.Request()
	if r.Method != http.MethodOptions || !strings.HasPrefix(r.URL.Path, ChatAPIPrefix) {
		return c.Next()
	}
	h := c.ResponseWriter().Header()
	h.Add("Vary", "Origin")
	if origin := r.Header.Get("Origin"); origin != "" && !strings.EqualFold(origin, "null") &&
		r.Header.Get("Access-Control-Request-Method") != "" {
		h.Set("Access-Control-Allow-Origin", origin)
		h.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Last-Event-ID")
		h.Set("Access-Control-Max-Age", "600")
	}
	c.ResponseWriter().WriteHeader(http.StatusNoContent)
	return nil
}
