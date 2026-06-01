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

package dto

// This file defines documentation-only payload shapes for the webhooks Posta
// POSTs to subscriber URLs.
type WebhookEvent struct {
	Event     string `json:"event"`
	EmailID   string `json:"email_id"`
	Timestamp string `json:"timestamp"`
}

// CampaignWebhookEvent is the payload for campaign lifecycle events
// (campaign.started, campaign.completed).
type CampaignWebhookEvent struct {
	Event      string `json:"event"`
	CampaignID uint   `json:"campaign_id"`
	Name       string `json:"name"`
	Timestamp  string `json:"timestamp"`
}

// ComplaintWebhookEvent is the payload for email.complained.
type ComplaintWebhookEvent struct {
	Event     string `json:"event"`
	EmailUUID string `json:"email_uuid"`
	Email     string `json:"email"`
	Timestamp string `json:"timestamp"`
}

// UnsubscribeWebhookEvent is the payload for email.unsubscribed.
type UnsubscribeWebhookEvent struct {
	Event     string `json:"event"`
	EmailUUID string `json:"email_uuid"`
	Email     string `json:"email"`
	ListID    *uint  `json:"list_id,omitempty"`
	Timestamp string `json:"timestamp"`
}

// InboundWebhookEvent is the payload for email.inbound.
type InboundWebhookEvent struct {
	Event       string                     `json:"event"`
	Timestamp   string                     `json:"timestamp"`
	InboundID   string                     `json:"inbound_id"`
	From        string                     `json:"from"`
	To          []string                   `json:"to"`
	Subject     string                     `json:"subject"`
	TextBody    string                     `json:"text_body,omitempty"`
	HTMLBody    string                     `json:"html_body,omitempty"`
	Headers     map[string]string          `json:"headers,omitempty"`
	Attachments []InboundWebhookAttachment `json:"attachments,omitempty"`
	Size        int64                      `json:"size"`
	MessageID   string                     `json:"message_id,omitempty"`
	Source      string                     `json:"source"`
	ReceivedAt  string                     `json:"received_at"`
}

// InboundWebhookAttachment describes an attachment in an InboundWebhookEvent.
type InboundWebhookAttachment struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	URL         string `json:"url,omitempty"`
}
