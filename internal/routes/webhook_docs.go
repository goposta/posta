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

	// Email lifecycle — generic {event, email_id, timestamp} payload.
	webhook("email.sent", "Fired when a message is accepted by the destination MTA", dto.WebhookEvent{})
	webhook("email.failed", "Fired when a message permanently fails after retries", dto.WebhookEvent{})

	// Reputation signals — richer recipient-scoped payloads.
	webhook("email.complained", "Fired when a recipient marks a message as spam", dto.ComplaintWebhookEvent{})
	webhook("email.unsubscribed", "Fired when a recipient opts out via one-click unsubscribe", dto.UnsubscribeWebhookEvent{})

	// Inbound mail — full parsed message.
	webhook("email.inbound", "Fired when an inbound email is received and parsed", dto.InboundWebhookEvent{})

	// Campaign lifecycle — {event, campaign_id, name, timestamp} payload.
	webhook("campaign.started", "Fired when a campaign begins sending", dto.CampaignWebhookEvent{})
	webhook("campaign.completed", "Fired when a campaign finishes sending", dto.CampaignWebhookEvent{})
}
