---
sidebar_position: 2
title: API Keys
description: Create and manage API keys
---

# API Keys

API keys provide programmatic access to Posta's email sending and status APIs.

:::tip
For the full request/response schema, see the interactive [API Reference](https://app.goposta.dev/docs).
:::

## Creating a Key

Create an API key from the dashboard or via `POST /api/v1/users/me/api-keys`. You can configure:

- **Name** — A human-readable label
- **Expiration** — Optional expiry date to enforce key rotation
- **IP allowlist** — Restrict usage to specific IPs or CIDR ranges (e.g., `203.0.113.0/24`)

:::warning
**Save the key immediately.** The full key is only shown once at creation time. Posta stores a hash of the key, not the key itself.
:::

## Using API Keys

Include the key in the `Authorization` header:

```bash
curl -X POST http://localhost:9000/api/v1/emails/send \
  -H "Authorization: Bearer posta_abc123def456..." \
  -H "Content-Type: application/json" \
  -d '{ ... }'
```

## Managing Keys

- **List** — View all keys (only prefix shown, never the full key)
- **Revoke** — Disable a key instantly without deleting it
- **Delete** — Permanently remove a key

## Security Features

- **Hashed storage** — Keys are stored as secure hashes with prefix for identification
- **Expiration** — Optional expiry dates to enforce key rotation
- **IP allowlist** — Restrict keys to specific IPs or CIDR ranges
- **Last used tracking** — Posta records when each key was last used
- **Revocation** — Instantly disable a key without deleting it
