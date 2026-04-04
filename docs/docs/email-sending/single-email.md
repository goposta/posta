---
sidebar_position: 1
title: Single Email
description: Send a single email via the API
---

# Single Email

Send a single email with HTML/text content, optional attachments, custom headers, and scheduled delivery.

:::tip
For the full request/response schema, see the interactive [API Reference](https://app.goposta.dev/docs).
:::

## Example

```bash
curl -X POST http://localhost:9000/api/v1/emails/send \
  -H "Authorization: Bearer your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "from": "sender@example.com",
    "to": ["recipient@example.com"],
    "subject": "Monthly Report",
    "html": "<h1>Report</h1><p>See attached.</p>",
    "text": "Report - See attached.",
    "headers": {
      "X-Campaign-ID": "report-2026-01"
    },
    "list_unsubscribe_url": "https://example.com/unsubscribe"
  }'
```

Response:

```json
{
  "success": true,
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "status": "queued"
  }
}
```

## Features

- **HTML and plain text** — Send both for maximum client compatibility
- **Custom headers** — Add arbitrary headers like `X-Campaign-ID`
- **List-Unsubscribe** — Include unsubscribe URL and one-click POST support
- **Scheduled delivery** — Set `send_at` to deliver at a specific time (see [Scheduled Email](/docs/email-sending/scheduled-email))
- **Attachments** — Attach base64-encoded files (see [Attachments](/docs/email-sending/attachments))
- **Dry run** — Add `?dry_run=true` to validate the request without sending

## Email Lifecycle

After sending, an email goes through these statuses:

```
pending → queued → processing → sent
                              → failed (with error_message)
```

- **pending** — Created, waiting to be queued
- **queued** — Added to the processing queue
- **processing** — Worker is sending via SMTP
- **sent** — Successfully delivered
- **failed** — Delivery failed (check `error_message`)
- **suppressed** — Blocked by suppression list
- **scheduled** — Waiting for `send_at` time
