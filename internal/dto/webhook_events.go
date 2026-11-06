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

// Documentation-only payload shapes for the webhooks Posta POSTs to subscriber URLs.
type WebhookEvent struct {
	Event     string `json:"event"`
	EmailID   string `json:"email_id"`
	Timestamp string `json:"timestamp"`
}

// CampaignWebhookEvent is the payload for campaign.started and campaign.completed.
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

type InboundWebhookAttachment struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	URL         string `json:"url,omitempty"`
}

// MessageWebhookEvent is the payload for message.received and message.spam.
type MessageWebhookEvent struct {
	Event       string                `json:"event"`
	Timestamp   string                `json:"timestamp"`
	MessageID   string                `json:"message_id"`
	FormID      string                `json:"form_id"`
	FormName    string                `json:"form_name"`
	SenderEmail string                `json:"sender_email,omitempty"`
	SenderName  string                `json:"sender_name,omitempty"`
	SenderPhone string                `json:"sender_phone,omitempty"`
	Subject     string                `json:"subject,omitempty"`
	Body        string                `json:"body,omitempty"`
	Fields      []MessageWebhookField `json:"fields,omitempty"`
	Status      string                `json:"status"`
	SpamScore   float64               `json:"spam_score"`
	ScanReasons []string              `json:"scan_reasons,omitempty"`
	ClientIP    string                `json:"client_ip,omitempty"`
	ReceivedAt  string                `json:"received_at"`
}

// MessageReplyWebhookEvent is the payload for message.replied (dashboard) and message.reply_received (email).
type MessageReplyWebhookEvent struct {
	Event          string `json:"event"`
	Timestamp      string `json:"timestamp"`
	MessageID      string `json:"message_id"`
	ReplyID        string `json:"reply_id"`
	FormID         string `json:"form_id"`
	FormName       string `json:"form_name"`
	Kind           string `json:"kind" enum:"operator,inbound"`
	From           string `json:"from"`
	To             string `json:"to"`
	Subject        string `json:"subject,omitempty"`
	TextBody       string `json:"text_body,omitempty"`
	HTMLBody       string `json:"html_body,omitempty"`
	EmailID        string `json:"email_id,omitempty" doc:"The outbound email carrying an operator reply, for delivery status."`
	SenderMismatch bool   `json:"sender_mismatch,omitempty" doc:"An inbound reply came from an address other than the original submitter."`
	CreatedAt      string `json:"created_at"`
}

type MessageWebhookField struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ChatWebhookEvent is the payload for the chat.* events. Message is set for
// chat.message.created, and for chat.conversation.started with the
// visitor's first message.
type ChatWebhookEvent struct {
	Event        string              `json:"event"`
	Timestamp    string              `json:"timestamp"`
	Conversation ChatWebhookConv     `json:"conversation"`
	Message      *ChatWebhookMessage `json:"message,omitempty"`
}

type ChatWebhookConv struct {
	ID           string             `json:"id"`
	WidgetID     string             `json:"widget_id"`
	WidgetName   string             `json:"widget_name"`
	State        string             `json:"state"`
	Status       string             `json:"status" doc:"Scan verdict on the first message: received or flagged"`
	AssignedToID *uint              `json:"assigned_to_id,omitempty"`
	StartPageURL string             `json:"start_page_url,omitempty"`
	Visitor      ChatWebhookVisitor `json:"visitor"`
	CreatedAt    string             `json:"created_at"`
}

type ChatWebhookVisitor struct {
	ID            string         `json:"id"`
	Name          string         `json:"name,omitempty"`
	Email         string         `json:"email,omitempty"`
	EmailVerified bool           `json:"email_verified" doc:"True only when the customer's server signed the identity; a typed email is never verified"`
	ExternalID    string         `json:"external_id,omitempty"`
	Attributes    map[string]any `json:"attributes,omitempty"`
}

type ChatWebhookMessage struct {
	ID          string                  `json:"id"`
	Seq         int64                   `json:"seq"`
	AuthorKind  string                  `json:"author_kind" doc:"visitor, agent or system"`
	AuthorID    *uint                   `json:"author_id,omitempty"`
	Body        string                  `json:"body"`
	Channel     string                  `json:"channel" doc:"chat, or email for a visitor's emailed answer"`
	Attachments []ChatWebhookAttachment `json:"attachments,omitempty"`
	CreatedAt   string                  `json:"created_at"`
}

type ChatWebhookAttachment struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}
