---
sidebar_position: 4
title: Scheduled Email
description: Schedule emails for future delivery
---

# Scheduled Email

Send emails at a specific future time using the `send_at` field on the single-send endpoint.

```
POST /api/v1/emails/send
```

## Usage

Include the `send_at` field with an ISO 8601 / RFC 3339 timestamp in the send request:

```bash
curl -X POST http://localhost:9000/api/v1/emails/send \
  -H "Authorization: Bearer your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "from": "sender@example.com",
    "to": ["recipient@example.com"],
    "subject": "Scheduled Reminder",
    "html": "<p>This is your scheduled reminder.</p>",
    "send_at": "2026-03-20T09:00:00Z"
  }'
```

The field is named `send_at` (a nullable timestamp). Use UTC (a `Z` suffix) to avoid timezone ambiguity.

## Response

```json
{
  "success": true,
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "status": "scheduled"
  }
}
```

## How scheduling works

When `send_at` is present and in the future, Posta stores the email with status `scheduled` and records the requested delivery time on the email record (`scheduled_at`). Instead of being enqueued for immediate delivery, the email is handed to the background worker with a delay equal to the time until `send_at`.

1. The send request is validated (sender, recipients, suppression checks) exactly as a normal send.
2. The email row is persisted with status `scheduled`.
3. A delivery task is enqueued with a processing delay (`send_at - now`). The task is held by the worker queue until that time arrives.
4. When the delay elapses, the worker picks up the task, the email transitions to `processing`, and it follows the normal SMTP delivery pipeline (`sent` on success, `failed` with an `error_message` on failure).

If `send_at` is in the past (or equal to now), the email is treated as an immediate send and goes straight to `queued`.

:::note
If the scheduling enqueue fails (for example, the queue backend is unavailable), the email is marked `failed` with an `error_message` describing the failure, and the API returns an error.
:::

## Viewing scheduled emails

Track a scheduled email by its UUID using the status endpoint. It reports `scheduled` until the delivery time arrives:

```bash
curl http://localhost:9000/api/v1/emails/550e8400-e29b-41d4-a716-446655440000/status \
  -H "Authorization: Bearer your-api-key"
```

See [Email Status](/docs/email-sending/email-status) for the full status list and response shape.

## Notes

- The `send_at` time must be in the future; otherwise the email is sent immediately.
- `send_at` is only available on the `/emails/send` endpoint (not on `/emails/send-template` or `/emails/batch`).
- There is no dedicated cancel endpoint for a scheduled email. Once accepted, the delivery task is queued for its `send_at` time.
