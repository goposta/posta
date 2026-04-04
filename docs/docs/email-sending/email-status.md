---
sidebar_position: 6
title: Email Status
description: Track email delivery status
---

# Email Status

Check the delivery status of a sent email by its UUID.

:::tip
For the full request/response schema, see the interactive [API Reference](https://app.goposta.dev/docs).
:::

## Example

```bash
curl http://localhost:9000/api/v1/emails/550e8400-e29b-41d4-a716-446655440000/status \
  -H "Authorization: Bearer your-api-key"
```

```json
{
  "success": true,
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "status": "sent",
    "error_message": null,
    "retry_count": 0,
    "created_at": "2026-01-01T00:00:00Z",
    "sent_at": "2026-01-01T00:00:01Z"
  }
}
```

## Status Values

| Status | Description |
|--------|-------------|
| `pending` | Created, waiting to be queued |
| `queued` | Added to the processing queue |
| `processing` | Worker is sending via SMTP |
| `sent` | Successfully delivered to SMTP server |
| `failed` | Delivery failed after all retries |
| `suppressed` | Blocked by suppression list |
| `scheduled` | Waiting for scheduled delivery time |

## Retry Failed Emails

Failed emails can be retried from the dashboard via `POST /api/v1/users/me/emails/{emailId}/retry`.

:::note
Only emails with `failed` status can be retried. Retry respects the configured maximum retry limit.
:::
