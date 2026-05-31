---
sidebar_position: 1
title: Overview
description: Receive, store, and forward inbound email with Posta
---

# Inbound Email

Inbound email lets Posta receive messages addressed to one of your verified domains, store them, and forward them to your application. Each received message is persisted and dispatched to your workspace webhook (as the `email.inbound` event) and pushed to a live SSE stream.

## Enabling Inbound

Inbound email is off by default. Enable it with configuration:

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTA_INBOUND_ENABLED` | `false` | Master switch for the inbound feature. |
| `POSTA_INBOUND_WEBHOOK_SECRET` | _(empty)_ | Opaque shared secret that authenticates the ingestion webhook. When empty, the webhook endpoint returns `403`. |
| `POSTA_INBOUND_MAX_MESSAGE_SIZE` | `26214400` (25 MiB) | Maximum raw message size in bytes. |
| `POSTA_INBOUND_MAX_ATTACH_SIZE` | `10485760` (10 MiB) | Maximum size per attachment in bytes. |

Posta also ships an SMTP listener (`POSTA_INBOUND_SMTP_*`) for receiving mail directly, but most deployments use an upstream MX provider that POSTs to the webhook endpoint documented here.

:::note
A message is only accepted if at least one recipient domain matches an ownership-verified [domain](../smtp-domains/domain-verification.md) in your account. Mail to unverified domains is rejected.
:::

## How It Works

```
provider/MX  ──►  POST /api/v1/inbound/webhook  ──►  stored (InboundEmail)
                                                          │
                                                          ├─►  email.inbound webhook  ──►  your endpoint
                                                          └─►  GET /inbound-stream (SSE)  ──►  dashboard / app
```

1. An upstream provider (or your own MX) normalizes a message and POSTs it to `POST /api/v1/inbound/webhook`, authenticating with the opaque secret.
2. Posta resolves the recipient to a verified domain, deduplicates by message id, and persists an `InboundEmail` record (raw `.eml` and attachments go to blob storage when configured).
3. The record is dispatched asynchronously as an `email.inbound` [webhook event](../webhooks/event-types.md) to your subscribed workspace webhooks, and an `email.inbound.received` event is published to the SSE stream.
4. You can browse, retry, download, and stream inbound mail from the workspace management API.

## Statuses

An inbound email moves through these statuses:

| Status | Meaning |
|--------|---------|
| `received` | Stored, awaiting (or in) dispatch. |
| `forwarded` | Successfully dispatched to your webhook. |
| `failed` | Webhook dispatch exhausted its retries. |
| `rejected` | Refused (e.g. suppressed sender or duplicate). |
| `quarantined` | Raw message could not be parsed. |

## Next Steps

- [Receiving Email](./receiving.md) — the ingestion webhook and attachment downloads.
- [Managing Inbound Email](./managing.md) — list, fetch, retry, export, and stream from your workspace.
