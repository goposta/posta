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

	"github.com/goposta/posta/internal/dto"
	"github.com/jkaninda/okapi"
)

func (r *Router) registerWebhookDocs() {

	sigHeader := okapi.DocHeader("X-Posta-Signature", "string",
		"HMAC-SHA256 of the raw request body, formatted as sha256=<hex>. "+
			"Verify it against the secret returned when the webhook was created.", true)
	uaHeader := okapi.DocHeader("User-Agent", "string", "Always \"Posta-Webhook/1.0\".", true)

	received := okapi.DocResponse(200, okapi.M{"received": true})

	webhook := func(name, summary string, body any) {
		r.app.Webhook(name, http.MethodPost,
			okapi.DocSummary(summary),
			okapi.DocTags("Webhooks"),
			sigHeader, uaHeader,
			okapi.DocRequestBody(body),
			received,
		)
	}

	for _, e := range dto.WebhookEvents {
		webhook(e.Name, e.Description, e.Payload)
	}
}
