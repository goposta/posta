---
sidebar_position: 5
title: Unsubscribe Lists
description: Transactional opt-out scopes for one-click unsubscribe links
---

# Unsubscribe Lists

An unsubscribe list is a **transactional opt-out scope**. A send can reference one of these by ID so Posta can mint a one-click unsubscribe link whose click suppresses the recipient on that list only — the recipient's other transactional mail (receipts, password resets) keeps flowing.

Unsubscribe lists are email-keyed and carry **no membership**: a recipient is never enrolled. The list is purely a suppression scope. This makes them distinct from [Subscriber Lists](/docs/subscribers/subscriber-lists), which are subscriber-keyed campaign audiences.

These routes are workspace-scoped. Send a JWT plus the `X-Posta-Workspace-Id` header (a workspace-scoped API key implies the workspace). Create, update, and delete require workspace edit permission.

## How it relates to sending

When a send names an unsubscribe list, the `{{ posta_unsubscribe_url }}` [system variable](/docs/templates/system-variables) renders a one-click link bound to that list and recipient. Clicking it writes a **list-scoped** [suppression](/docs/contacts/suppression-list) (`kind: "list_unsubscribe"`) for that address against this list — not a global block. Future sends that reference the same list skip the address; mail on other lists is unaffected.

## Fields

| Field | Description |
|-------|-------------|
| `id` | Numeric ID used by the authenticated API |
| `uuid` | Opaque, non-enumerable public handle (used in hosted pages / webhook payloads) |
| `name` | Internal label, unique per workspace (**required**) |
| `public_name` | Name shown to recipients on the unsubscribe page; falls back to `name` when empty |
| `description` | Optional description |
| `active` | Whether the list is active (defaults to `true`) |
| `created_at` / `updated_at` | Timestamps |

## Create an Unsubscribe List

```
POST /api/v1/workspaces/current/unsubscribe-lists
```

| Field | Required | Description |
|-------|----------|-------------|
| `name` | yes | Internal label, unique per workspace |
| `public_name` | no | Name shown to recipients |
| `description` | no | Optional description |

```bash
curl -X POST http://localhost:9000/api/v1/workspaces/current/unsubscribe-lists \
  -H "Authorization: Bearer <jwt>" \
  -H "X-Posta-Workspace-Id: 1" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "product-updates",
    "public_name": "Product updates",
    "description": "Feature announcements and changelog emails"
  }'
```

New lists are created with `active: true`. Returns `201 Created`, or `409 Conflict` if a list with that name already exists in the workspace.

## List Unsubscribe Lists

```
GET /api/v1/workspaces/current/unsubscribe-lists?page=0&size=20
```

```json
{
  "success": true,
  "data": [
    {
      "id": 1,
      "uuid": "8f3c...",
      "user_id": 1,
      "workspace_id": 1,
      "name": "product-updates",
      "public_name": "Product updates",
      "description": "Feature announcements and changelog emails",
      "active": true,
      "created_at": "2026-01-01T00:00:00Z",
      "updated_at": null
    }
  ]
}
```

## Get an Unsubscribe List

```
GET /api/v1/workspaces/current/unsubscribe-lists/{id}
```

Returns the list, or `404` if it does not exist in the current workspace.

## Update an Unsubscribe List

```
PUT /api/v1/workspaces/current/unsubscribe-lists/{id}
```

All fields are optional; only the ones you send are changed. Set `active` to `false` to deactivate a list without deleting it.

```json
{
  "public_name": "Product news",
  "active": false
}
```

## Delete an Unsubscribe List

```
DELETE /api/v1/workspaces/current/unsubscribe-lists/{id}
```

Returns `204 No Content`.
