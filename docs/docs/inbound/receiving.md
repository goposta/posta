---
sidebar_position: 2
title: Receiving Email
description: Ingest inbound messages via the public webhook and download attachments
---

# Receiving Email

Upstream providers (or your own MX) deliver normalized messages to Posta's public ingestion webhook. These endpoints are authenticated with the opaque inbound secret, not a JWT or API key.

## Ingestion Webhook

```
POST /api/v1/inbound/webhook
```

Authenticate with the shared secret from `POSTA_INBOUND_WEBHOOK_SECRET`, sent in the `X-Posta-Inbound-Secret` header. The comparison is constant-time. If inbound is not configured (empty secret), the endpoint returns `403`.

### Request Body

The payload is a normalized JSON envelope under a top-level `body` object:

```json
{
  "body": {
    "from": "sender@example.com",
    "to": ["inbox@yourdomain.com"],
    "subject": "Hello",
    "text": "Plain text body",
    "html": "<p>HTML body</p>",
    "headers": { "X-Custom": "value" },
    "message_id": "<abc123@example.com>",
    "spam_score": 0.1,
    "attachments": [
      {
        "filename": "invoice.pdf",
        "content_type": "application/pdf",
        "content": "<base64-encoded bytes>"
      }
    ],
    "raw": "<optional base64-encoded raw RFC 5322 message>"
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `from` | string | yes | Sender address. |
| `to` | array | yes | Recipient addresses. At least one must match a verified domain. |
| `subject` | string | no | Message subject. |
| `text` | string | no | Plain-text body. |
| `html` | string | no | HTML body. |
| `headers` | object | no | Map of header name to value. |
| `message_id` | string | no | RFC 5322 Message-ID; angle brackets are stripped. Used for deduplication. |
| `spam_score` | number | no | Provider-supplied spam score. |
| `attachments` | array | no | Attachments with base64-encoded `content`. |
| `raw` | string | no | Base64-encoded raw `.eml`, stored for later export. |

```bash
curl -X POST http://localhost:9000/api/v1/inbound/webhook \
  -H "X-Posta-Inbound-Secret: <inbound_secret>" \
  -H "Content-Type: application/json" \
  -d '{
    "body": {
      "from": "sender@example.com",
      "to": ["inbox@yourdomain.com"],
      "subject": "Hello",
      "text": "Plain text body"
    }
  }'
```

### Response

On acceptance the endpoint returns `202 Accepted`:

```json
{
  "accepted": true,
  "inbound_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "received"
}
```

The `status` field and HTTP code vary by outcome:

| HTTP | `status` | Condition |
|------|----------|-----------|
| `202` | `received` | Message accepted and queued for dispatch. |
| `200` | `duplicate` | A message with the same id was already ingested (idempotent). |
| `202` | `suppressed` | Sender is on the recipient's suppression list; stored, not forwarded. |
| `403` | — | Recipient domain is not verified, or inbound is not configured. |
| `413` | `too_large` | Message or attachment exceeds the configured size limit. |
| `401` | — | Invalid inbound secret. |

### Deduplication

Posta deduplicates by `message_id` per resolved user. If no `message_id` is supplied, a stable content hash (sender, recipients, subject, and size) is used as a fallback. Re-POSTing a duplicate returns `200` with `status: "duplicate"` and the original `inbound_id`.

## Downloading Attachments (Signed Token)

When the `email.inbound` webhook is dispatched, each attachment includes a signed download URL rather than inline bytes. Webhook consumers fetch the content asynchronously:

```
GET /api/v1/inbound/attachments/{uuid}/{idx}?t={token}
```

- `{uuid}` — the inbound email UUID.
- `{idx}` — the zero-based attachment index.
- `t` — the HMAC-signed token from the webhook payload.

The token is validated against the server's HMAC key; an invalid or missing token returns `401`. The response streams the raw attachment bytes with the original `Content-Type` and a `Content-Disposition` filename.

:::note
This token-signed endpoint is for webhook consumers. Authenticated workspace users can download attachments without a token via the [management API](./managing.md#download-an-attachment).
:::
