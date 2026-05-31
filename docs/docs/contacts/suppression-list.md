---
sidebar_position: 4
title: Suppression List
description: Manage suppressed email addresses
---

# Suppression List

The suppression list prevents emails from being sent to specific addresses. Addresses are added automatically on hard bounces/complaints, or manually.

These routes are workspace-scoped. Send a JWT plus the `X-Posta-Workspace-Id` header (a workspace-scoped API key implies the workspace).

A suppression can be **global** (blocks all mail to the address) or **list-scoped** (an opt-out of a single [Unsubscribe List](/docs/contacts/unsubscribe-lists)). Pass `list_id` to scope a suppression to one list; omit it for a global block.

## Add to Suppression List

```
POST /api/v1/workspaces/current/suppressions
```

| Field | Required | Description |
|-------|----------|-------------|
| `email` | yes | Address to suppress |
| `reason` | no | Free-text note |
| `list_id` | no | Scope the suppression to one unsubscribe list (omit for a global block) |

```bash
curl -X POST http://localhost:9000/api/v1/workspaces/current/suppressions \
  -H "Authorization: Bearer <jwt>" \
  -H "X-Posta-Workspace-Id: 1" \
  -H "Content-Type: application/json" \
  -d '{"email": "unsubscribed@example.com", "reason": "User requested removal"}'
```

Manually created suppressions are stored with `kind: "manual"`. Returns `409 Conflict` if already suppressed.

## List Suppressed Addresses

```
GET /api/v1/workspaces/current/suppressions?page=0&size=20&list_id=0
```

Pass `list_id` (a positive integer) to return only suppressions scoped to that unsubscribe list. Omit it (or use `0`) to list all.

Response:

```json
{
  "success": true,
  "data": [
    {
      "id": 1,
      "user_id": 1,
      "workspace_id": 1,
      "email": "unsubscribed@example.com",
      "list_id": null,
      "kind": "manual",
      "reason": "User requested removal",
      "created_at": "2026-01-01T00:00:00Z"
    }
  ]
}
```

The `kind` field classifies why an address is suppressed: `hard`, `bounce`, `complaint`, `list_unsubscribe`, or `manual`.

## Remove from Suppression List

```
DELETE /api/v1/workspaces/current/suppressions
```

```json
{
  "email": "resubscribed@example.com"
}
```

Include `list_id` to remove only the list-scoped suppression; omit it to remove the global block. Returns `204 No Content`.

## How Suppression Works

When sending an email:

1. Posta checks the recipient against the suppression list
2. If suppressed, the email is marked as `suppressed` and not delivered
3. In batch sends, suppressed recipients are skipped and reported in the response

```json
{
  "results": [
    {"email": "active@example.com", "id": "uuid", "status": "queued"},
    {"email": "suppressed@example.com", "status": "suppressed"}
  ]
}
```
