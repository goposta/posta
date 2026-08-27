// SPDX-FileCopyrightText: 2026 Jonas Kaninda
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
