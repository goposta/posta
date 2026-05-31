---
sidebar_position: 2
title: API Keys
description: Create and manage API keys
---

# API Keys

API keys provide programmatic access to Posta's email sending and status APIs. Keys are scoped to a workspace and managed under `/api/v1/workspaces/current/api-keys`.

:::tip
For the full request/response schema, see the interactive [API Reference](https://app.goposta.dev/docs).
:::

## Creating a Key

```
POST /api/v1/workspaces/current/api-keys
```

Requires a JWT bearer token and the `X-Posta-Workspace-Id` header.

```bash
curl -X POST http://localhost:9000/api/v1/workspaces/current/api-keys \
  -H "Authorization: Bearer <jwt>" \
  -H "X-Posta-Workspace-Id: 1" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Production sender",
    "allowed_ips": ["203.0.113.0/24"],
    "expires_in_days": 90
  }'
```

Request body:

| Field | Type | Description |
|-------|------|-------------|
| `name` | string (required) | Human-readable label |
| `allowed_ips` | string[] | Restrict usage to specific IPs or CIDR ranges |
| `expires_in_days` | integer | Days until expiry; `0` or omitted means never expires |

Response (201):

```json
{
  "success": true,
  "data": {
    "key": "posta_abc123def456...",
    "id": 42,
    "name": "Production sender",
    "prefix": "posta_abc1",
    "expires_at": "2026-08-30T00:00:00Z",
    "message": "Save this key securely. It will not be shown again."
  }
}
```

:::warning
**Save the key immediately.** The full key is only shown once at creation time. Posta stores a hash of the key, not the key itself.
:::

## Using API Keys

Include the key in the `Authorization` header when sending emails or checking status:

```bash
curl -X POST http://localhost:9000/api/v1/emails/send \
  -H "Authorization: Bearer posta_abc123def456..." \
  -H "Content-Type: application/json" \
  -d '{ ... }'
```

A workspace-scoped API key already implies the workspace, so the `X-Posta-Workspace-Id` header is not needed for email-sending calls.

## Listing Keys

```
GET /api/v1/workspaces/current/api-keys
```

Returns paginated results. The full key is never returned — only the prefix is shown for identification.

## Revoking a Key

Instantly disable a key without deleting it:

```
PUT /api/v1/workspaces/current/api-keys/{id}/revoke
```

## Deleting a Key

Permanently remove a key. Only expired or already-revoked keys can be deleted — revoke an active key first.

```
DELETE /api/v1/workspaces/current/api-keys/{id}
```

## Security Features

- **Hashed storage** — Keys are stored as secure hashes; only the prefix is kept for identification
- **Expiration** — Optional expiry dates to enforce key rotation
- **IP allowlist** — Restrict keys to specific IPs or CIDR ranges
- **Last used tracking** — Posta records when each key was last used
- **Revocation** — Instantly disable a key without deleting it
