---
sidebar_position: 2
title: Event Types
description: Available webhook event types
---

# Event Types

## Email Events

| Event | Description |
|-------|-------------|
| `email.sent` | Email was successfully delivered to the SMTP server |
| `email.failed` | Email delivery failed after all retries |
| `email.unsubscribed` | Recipient unsubscribed via a one-click link |
| `email.complained` | Recipient filed a spam complaint |
| `email.inbound` | An inbound email was received (see [Inbound Email](#inbound-email)) |

## Campaign Events

| Event | Description |
|-------|-------------|
| `campaign.started` | A campaign began sending |
| `campaign.completed` | A campaign finished sending all messages |

## Payload Shapes

### `email.sent`

Fired when an email is successfully delivered to the SMTP server:

```json
{
  "event": "email.sent",
  "email_id": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2026-01-01T00:00:01Z"
}
```

### `email.failed`

Fired when delivery fails after all retries:

```json
{
  "event": "email.failed",
  "email_id": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2026-01-01T00:00:01Z"
}
```

### `email.unsubscribed`

Fired when a recipient opts out via a one-click unsubscribe link:

```json
{
  "event": "email.unsubscribed",
  "email_uuid": "550e8400-e29b-41d4-a716-446655440000",
  "email": "recipient@example.com",
  "list_id": 7,
  "timestamp": "2026-01-01T00:00:01Z"
}
```

`list_id` is omitted when the unsubscribe is not scoped to a specific subscriber list.

### `email.complained`

Fired when a spam complaint is recorded (e.g. via the bounce webhook):

```json
{
  "event": "email.complained",
  "email_uuid": "550e8400-e29b-41d4-a716-446655440000",
  "email": "recipient@example.com",
  "timestamp": "2026-01-01T00:00:01Z"
}
```

### `campaign.started`

Fired when a campaign begins sending:

```json
{
  "event": "campaign.started",
  "campaign_id": 42,
  "name": "Spring Newsletter",
  "timestamp": "2026-01-01T00:00:01Z"
}
```

### `campaign.completed`

Fired when a campaign finishes sending all of its messages:

```json
{
  "event": "campaign.completed",
  "campaign_id": 42,
  "name": "Spring Newsletter",
  "timestamp": "2026-01-01T00:00:01Z"
}
```

## Inbound Email

The `email.inbound` event fires when Posta receives an incoming message on a verified domain. Its payload is richer than the outbound events:

```json
{
  "event": "email.inbound",
  "timestamp": "2026-01-01T00:00:01Z",
  "inbound_id": "7d3f9a12-...",
  "from": "sender@example.com",
  "to": ["inbox@yourdomain.com"],
  "subject": "Hello",
  "text_body": "Plain text body",
  "html_body": "<p>HTML body</p>",
  "headers": { "Reply-To": "sender@example.com" },
  "attachments": [
    {
      "filename": "report.pdf",
      "content_type": "application/pdf",
      "size": 12345,
      "url": "https://..."
    }
  ],
  "size": 14200,
  "message_id": "<unique@mail.example.com>",
  "source": "smtp",
  "received_at": "2026-01-01T00:00:00Z"
}
```

For configuring inbound routing and managing received messages, see the Inbound Email section in the sidebar.
