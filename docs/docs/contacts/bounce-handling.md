---
sidebar_position: 3
title: Bounce Handling
description: Track and manage email bounces
---

# Bounce Handling

Posta tracks email bounces and automatically suppresses addresses that hard bounce.

## Bounce Types

| Type | Description | Action |
|------|-------------|--------|
| `hard` | Permanent failure (e.g., mailbox doesn't exist) | Auto-suppressed |
| `soft` | Temporary failure (e.g., mailbox full) | Tracked, not suppressed |
| `complaint` | Recipient reported as spam | Auto-suppressed |

These routes are workspace-scoped. Send a JWT plus the `X-Posta-Workspace-Id` header (a workspace-scoped API key implies the workspace).

## Record a Bounce

```
POST /api/v1/workspaces/current/bounces
```

| Field | Required | Description |
|-------|----------|-------------|
| `email_id` | yes | UUID of the sent email this bounce relates to |
| `recipient` | yes | Bounced recipient address |
| `type` | yes | One of `hard`, `soft`, `complaint` |
| `reason` | no | Free-text diagnostic / SMTP message |

```bash
curl -X POST http://localhost:9000/api/v1/workspaces/current/bounces \
  -H "Authorization: Bearer <jwt>" \
  -H "X-Posta-Workspace-Id: 1" \
  -H "Content-Type: application/json" \
  -d '{
    "email_id": "b1e3...uuid",
    "recipient": "bounced@example.com",
    "type": "hard",
    "reason": "550 5.1.1 The email account does not exist"
  }'
```

Returns `201 Created` with the recorded bounce. An invalid `type` returns `400`, and an unknown `email_id` returns `404`.

## List Bounces

```
GET /api/v1/workspaces/current/bounces?page=0&size=20
```

Each item includes `id`, `email_id`, `recipient`, `type`, `reason`, and `created_at`.

## Automatic Suppression

When a **hard bounce** or **complaint** is recorded:

1. The email address is automatically added to the [suppression list](/docs/contacts/suppression-list)
2. Future sends to that address are blocked with status `suppressed`
3. Batch sends skip suppressed addresses automatically

**Soft bounces** are tracked but do not trigger automatic suppression. They may resolve on their own (e.g., when a full mailbox is cleared).
