---
sidebar_position: 1
title: Overview
description: Real-time webhooks for email events
---

# Webhooks

Receive real-time HTTP notifications when email events occur. Posta sends POST requests to your configured URLs with event details.

:::tip
For the full request/response schema, see the interactive [API Reference](https://app.goposta.dev/docs).
:::

## Creating a Webhook

Webhooks are workspace-scoped. Create one via the dashboard or the API:

```
POST /api/v1/workspaces/current/webhooks
```

```bash
curl -X POST http://localhost:9000/api/v1/workspaces/current/webhooks \
  -H "Authorization: Bearer <jwt>" \
  -H "X-Posta-Workspace-Id: 1" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://your-app.com/webhooks/posta",
    "events": ["email.sent", "email.failed", "campaign.completed"]
  }'
```

Request body:

| Field | Type | Description |
|-------|------|-------------|
| `url` | string | HTTPS endpoint to receive events |
| `events` | array | One or more [event types](./event-types.md) to subscribe to |
| `filters` | array | Optional sender-address or domain filters |

:::note
The signing `secret` is generated automatically by Posta and returned in the creation response. Store it securely — it is used to verify the `X-Posta-Signature` header on every delivery.
:::

## Listing and Deleting Webhooks

```
GET    /api/v1/workspaces/current/webhooks
DELETE /api/v1/workspaces/current/webhooks/{id}
```

## Webhook Payload

Posta sends a POST request with a JSON payload. For outbound email events the shape is:

```json
{
  "event": "email.sent",
  "email_id": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2026-01-01T00:00:01Z"
}
```

For campaign events:

```json
{
  "event": "campaign.completed",
  "campaign_id": 42,
  "name": "Spring Newsletter",
  "timestamp": "2026-01-01T00:00:01Z"
}
```

See [Event Types](./event-types.md) for payload shapes of all events.

## Signature Verification

Posta generates a random signing secret per webhook and returns it in the creation response. Every delivery includes an HMAC-SHA256 signature header:

```
X-Posta-Signature: sha256=abc123...
```

Verify the signature in your webhook handler to ensure the request is from Posta:

```go
mac := hmac.New(sha256.New, []byte(webhookSecret))
mac.Write(requestBody)
expectedSignature := hex.EncodeToString(mac.Sum(nil))
```

## Retry Behavior

- Failed webhook deliveries are retried up to `POSTA_WEBHOOK_MAX_RETRIES` times (default: 3)
- Retries use exponential backoff
- Request timeout: `POSTA_WEBHOOK_TIMEOUT_SECS` (default: 10 seconds)
- Any `2xx` status code is considered successful
