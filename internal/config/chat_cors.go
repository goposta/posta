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

const ChatAPIPrefix = "/api/v1/chat/"

// ChatPreflight allows any origin: every real chat request checks its widget's allowlist and no credentials are used.
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
