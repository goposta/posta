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

package dto

// WebhookEventDef describes one event a webhook can subscribe to.
type WebhookEventDef struct {
	Name        string
	Description string
	Payload     any
}

// WebhookEvents is the single list of subscribable webhook events. The API
// docs are generated from it, and a test pins the webhook request's enum tag
// to it, so an event cannot be documented but unsubscribable or the reverse.
var WebhookEvents = []WebhookEventDef{
	// Email lifecycle — generic {event, email_id, timestamp} payload.
	{"email.sent", "Fired when a message is accepted by the destination MTA", WebhookEvent{}},
	{"email.failed", "Fired when a message permanently fails after retries", WebhookEvent{}},
	// Inbound mail — full parsed message.
	{"email.inbound", "Fired when an inbound email is received and parsed", InboundWebhookEvent{}},
	// Reputation signals — richer recipient-scoped payloads.
	{"email.unsubscribed", "Fired when a recipient opts out via one-click unsubscribe", UnsubscribeWebhookEvent{}},
	{"email.complained", "Fired when a recipient marks a message as spam", ComplaintWebhookEvent{}},
	// Web form messages — full submission plus the scan verdict, and thread replies.
	{"message.received", "Fired when a web form submission passes scanning", MessageWebhookEvent{}},
	{"message.spam", "Fired when a web form submission is quarantined or rejected by scanning", MessageWebhookEvent{}},
	{"message.replied", "Fired when an operator replies to a web form message from the dashboard", MessageReplyWebhookEvent{}},
	{"message.reply_received", "Fired when the sender answers a web form thread by email", MessageReplyWebhookEvent{}},
	// Campaign lifecycle — {event, campaign_id, name, timestamp} payload.
	{"campaign.started", "Fired when a campaign begins sending", CampaignWebhookEvent{}},
	{"campaign.completed", "Fired when a campaign finishes sending", CampaignWebhookEvent{}},
}

// WebhookEventNames returns the subscribable event names in registry order.
func WebhookEventNames() []string {
	names := make([]string, len(WebhookEvents))
	for i, e := range WebhookEvents {
		names[i] = e.Name
	}
	return names
}
