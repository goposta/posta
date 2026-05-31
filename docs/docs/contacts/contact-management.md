---
sidebar_position: 1
title: Contact Management
description: Track email recipients
---

# Contact Management

Posta automatically tracks recipients when you send emails. Contacts are **read-only**: they are created and updated by the system, and the API exposes only list and detail reads — there are no create, update, or delete endpoints.

:::note
To build reusable mailing lists or manage opt-out groups, use [Subscriber Lists](/docs/subscribers/subscriber-lists) and [Unsubscribe Lists](/docs/contacts/unsubscribe-lists). Contacts are not manually grouped.
:::

These routes are workspace-scoped. Send a JWT plus the `X-Posta-Workspace-Id` header (a workspace-scoped API key implies the workspace, so the header is not needed in that case).

## List Contacts

```
GET /api/v1/workspaces/current/contacts?page=0&size=20&search=alice
```

| Parameter | Description |
|-----------|-------------|
| `page` | Page number (zero-based, default `0`) |
| `size` | Items per page (default `20`) |
| `search` | Search by email or name |

```bash
curl http://localhost:9000/api/v1/workspaces/current/contacts \
  -H "Authorization: Bearer <jwt>" \
  -H "X-Posta-Workspace-Id: 1"
```

Response:

```json
{
  "success": true,
  "data": [
    {
      "id": 1,
      "user_id": 1,
      "workspace_id": 1,
      "email": "alice@example.com",
      "name": "Alice",
      "sent_count": 42,
      "fail_count": 1,
      "suppressed": false,
      "last_sent_at": "2026-01-15T10:00:00Z",
      "created_at": "2026-01-01T00:00:00Z"
    }
  ]
}
```

## Contact Details

```
GET /api/v1/workspaces/current/contacts/{id}
```

Returns a single contact by numeric ID, with the same fields as the list response. The `suppressed` flag reflects whether the contact's email is currently on the [suppression list](/docs/contacts/suppression-list).

## Automatic Tracking

Contacts are created automatically when you send to a new email address. Posta tracks:

- **`sent_count`** — Total emails successfully sent
- **`fail_count`** — Total delivery failures
- **`suppressed`** — Whether the contact's email is currently suppressed (computed at read time, not stored on the contact)
- **`last_sent_at`** — Timestamp of the most recent email
