---
sidebar_position: 3
title: Managing Inbound Email
description: List, fetch, retry, export, and stream inbound email from your workspace
---

# Managing Inbound Email

These workspace-scoped endpoints let you browse and operate on received mail. All routes require a JWT and the workspace header:

```
Authorization: Bearer <jwt>
X-Posta-Workspace-Id: 1
```

## List Inbound Emails

```
GET /api/v1/workspaces/current/inbound-emails
```

Returns a paginated list scoped to the current workspace.

Query parameters:

| Param | Type | Description |
|-------|------|-------------|
| `page` | int | Page number (default `0`). |
| `size` | int | Page size (default `20`). |
| `status` | string | Filter by `received`, `forwarded`, `failed`, `rejected`, or `quarantined`. |
| `source` | string | Filter by `smtp` or `webhook`. |
| `sender` | string | Substring match on sender address (case-insensitive). |
| `q` | string | Full-text search on subject. |

```bash
curl http://localhost:9000/api/v1/workspaces/current/inbound-emails?status=received \
  -H "Authorization: Bearer <jwt>" \
  -H "X-Posta-Workspace-Id: 1"
```

## Get an Inbound Email

```
GET /api/v1/workspaces/current/inbound-emails/{id}
```

`{id}` is the inbound email UUID. Returns the full record:

```json
{
  "id": 123,
  "uuid": "550e8400-e29b-41d4-a716-446655440000",
  "user_id": 1,
  "workspace_id": 1,
  "domain_id": 7,
  "message_id": "abc123@example.com",
  "sender": "sender@example.com",
  "recipients": ["inbox@yourdomain.com"],
  "subject": "Hello",
  "text_body": "Plain text body",
  "html_body": "<p>HTML body</p>",
  "size": 2048,
  "spam_score": 0.1,
  "status": "forwarded",
  "source": "webhook",
  "retry_count": 0,
  "received_at": "2026-01-01T00:00:00Z",
  "forwarded_at": "2026-01-01T00:00:02Z",
  "created_at": "2026-01-01T00:00:00Z"
}
```

Attachment metadata and headers are stored internally (`attachments_json`, `headers_json`) and may be present when set. Fetch attachment bytes via the download endpoint below.

## Delete an Inbound Email

```
DELETE /api/v1/workspaces/current/inbound-emails/{id}
```

Removes the record and best-effort deletes its blob-stored raw message and attachments. Returns `204 No Content`.

## Retry Dispatch

```
POST /api/v1/workspaces/current/inbound-emails/{id}/retry
```

Re-enqueues a message for processing. `quarantined` records are re-run through the MIME parse pipeline; `failed` and stuck `received` records are re-dispatched to your webhook. Only `failed`, `quarantined`, or stuck `received` messages can be retried — others return `400`.

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "received"
}
```

:::note
Retry requires the async worker to be running; without it the endpoint returns `403`.
:::

## Download the Raw Message

```
GET /api/v1/workspaces/current/inbound-emails/{id}/raw
```

Streams the raw RFC 5322 message as `message/rfc822` with a `.eml` filename. Returns `404` if the raw bytes were not stored.

## Download an Attachment

```
GET /api/v1/workspaces/current/inbound-emails/{uuid}/attachments/{idx}
```

Streams an attachment by zero-based index for an inbound email you own. Unlike the [public signed-token endpoint](./receiving.md#downloading-attachments-signed-token), this authenticated route needs no token. The response uses the attachment's original `Content-Type` and filename.

## Live Stream (SSE)

```
GET /api/v1/workspaces/current/inbound-stream
```

A Server-Sent Events stream of inbound-email events for the current user. Because `EventSource` cannot set headers, this endpoint authenticates with the JWT as a **query parameter** instead of the `Authorization` header.

Events emitted: `email.inbound.received`, `email.inbound.forwarded`, and `email.inbound.failed`. An initial `system.info` event is sent on connect, and the server pings every 30 seconds to keep the connection alive. Events from other users' inboxes are filtered out.

```bash
curl -N "http://localhost:9000/api/v1/workspaces/current/inbound-stream?token=<jwt>&workspace_id=1"
```

Each event's `data` carries the inbound details, including `inbound_id`, `sender`, `recipients`, `subject`, `source`, `size`, and `workspace_id`.
