---
sidebar_position: 3
title: Batch Email
description: Send emails to multiple recipients efficiently
---

# Batch Email

Send template-based emails to multiple recipients in a single API call, with per-recipient variable customization.

:::tip
For the full request/response schema, see the interactive [API Reference](https://app.goposta.dev/docs).
:::

## Example

```bash
curl -X POST http://localhost:9000/api/v1/emails/batch \
  -H "Authorization: Bearer your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "template": "newsletter",
    "from": "news@example.com",
    "language": "en",
    "recipients": [
      {
        "email": "bob@example.com",
        "template_data": {"name": "Bob", "plan": "Pro"}
      },
      {
        "email": "carol@example.com",
        "language": "fr",
        "template_data": {"name": "Carol", "plan": "Enterprise"}
      },
      {
        "email": "dave@example.com",
        "template_data": {"name": "Dave", "plan": "Free"}
      }
    ]
  }'
```

## Batch Behavior

- Each recipient is processed independently — a failure for one does not affect others
- Suppressed addresses are automatically skipped
- Per-recipient language overrides the batch-level default
- Rate limits apply to the total number of emails in the batch
- Each email gets its own UUID for status tracking

## Result Statuses

| Status | Description |
|--------|-------------|
| `queued` | Successfully queued for delivery |
| `suppressed` | Recipient is on the suppression list |
| `failed` | Failed to queue (see `error` field) |
