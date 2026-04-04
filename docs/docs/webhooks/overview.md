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

Create a webhook from the dashboard or via `POST /api/v1/users/me/webhooks`:

```json
{
  "url": "https://your-app.com/webhooks/posta",
  "events": ["email.sent", "email.failed", "email.bounced"],
  "secret": "your-webhook-secret"
}
```

## Webhook Payload

Posta sends a POST request with a JSON payload:

```json
{
  "event": "email.sent",
  "timestamp": "2026-01-01T00:00:01Z",
  "data": {
    "email_id": "550e8400-e29b-41d4-a716-446655440000",
    "to": "recipient@example.com",
    "status": "sent"
  }
}
```

## Signature Verification

If a `secret` is configured, Posta includes an HMAC-SHA256 signature in the request header:

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
