---
sidebar_position: 1
title: Overview
description: Multi-tenant workspaces, roles, and the workspace context header
---

# Workspaces

Workspaces provide multi-tenant isolation within Posta. Every resource — templates, SMTP servers, domains, contacts, API keys — belongs to exactly one workspace, and a workspace is shared with as many or as few people as you invite.

## Concepts

### Personal space

A workspace is provisioned for you when you sign up, named after you and owned by you. It is an ordinary workspace in every respect: rename it, invite people to it, or delete it once you have another. There is no separate "personal" workspace type.

One workspace is special. The **system workspace** is created on first boot, owned by the first administrator, and holds platform-managed resources. It is flagged `system: true`, admits only platform administrators, and cannot be renamed or deleted.

### Workspaces

A workspace is an isolated environment where team members collaborate. Resources created within a workspace are only visible to members of that workspace. Each workspace has:

- A unique **name** and **slug** (a URL-friendly identifier)
- An **owner** (the creator)
- **Members**, each with a role
- A **default language**
- Isolated operational resources (templates, SMTP servers, domains, API keys, contacts, subscribers, campaigns, emails, webhooks, etc.)

### Roles

Posta defines four workspace roles. Permissions are cumulative:

| Role | View resources | Create / edit resources | Manage members & invitations | Delete workspace |
|------|:---:|:---:|:---:|:---:|
| **owner** | Yes | Yes | Yes | Yes |
| **admin** | Yes | Yes | Yes | No |
| **editor** | Yes | Yes | No | No |
| **viewer** | Yes | No | No | No |

There is exactly one owner per workspace: the creator. The owner role cannot be assigned through invitations or role updates, and the owner cannot be removed or have their role changed.

:::note
Operational settings, the workspace audit log, and the GDPR data-management endpoints require the **admin** level or higher (owner or admin). Resource read/write follows the table above.
:::

## API usage

Workspace management uses **JWT bearer authentication** (the dashboard/UI token), not API keys.

```
Authorization: Bearer <jwt>
```

### Workspace context header

Routes under `/api/v1/workspaces/current/*` operate against the **active workspace**, which is resolved from the `X-Posta-Workspace-Id` header:

```
X-Posta-Workspace-Id: 1
```

```bash
curl -X GET http://localhost:9000/api/v1/workspaces/current/templates \
  -H "Authorization: Bearer <jwt>" \
  -H "X-Posta-Workspace-Id: 1"
```

The header value is the numeric workspace ID. If you are not a member of that workspace the request is rejected with `403`. The header is **required** for every `/workspaces/current/*` route; omitting it returns `400 X-Posta-Workspace-Id header is required`.

:::caution
The correct header is `X-Posta-Workspace-Id`. Earlier drafts referred to `X-Workspace-ID` — that name is wrong and is not recognized by the API.
:::

Omit the header and the request operates on your **default workspace** — the one you last set with `PUT /api/v1/users/me/default-workspace`, or the oldest one you belong to. The role you hold in that workspace applies, so a viewer stays a viewer on a header-less request.

### Workspace-scoped API keys

API keys created inside a workspace context are bound to that workspace. When you authenticate with such a key, the active workspace is implied by the key itself, so the `X-Posta-Workspace-Id` header is **not** needed:

```bash
curl -X POST http://localhost:9000/api/v1/emails/send \
  -H "Authorization: Bearer <workspace_api_key>" \
  -H "Content-Type: application/json" \
  -d '{ ... }'
```

This is how transactional sending and other API-key endpoints stay scoped to the right workspace without a header.

## Workspace management endpoints

All endpoints below use JWT auth. Those operating on `/current` additionally require the `X-Posta-Workspace-Id` header.

| Method | Path | Header required | Description |
|--------|------|:---:|-------------|
| `POST` | `/api/v1/workspaces` | No | Create a workspace (creator becomes owner) |
| `GET` | `/api/v1/workspaces` | No | List workspaces the user belongs to |
| `GET` | `/api/v1/workspaces/current` | Yes | Get the active workspace |
| `PUT` | `/api/v1/workspaces/current` | Yes | Update name / description / default language |
| `DELETE` | `/api/v1/workspaces/current` | Yes | Delete the workspace |
| `GET` | `/api/v1/workspaces/current/members` | Yes | List members |
| `PUT` | `/api/v1/workspaces/current/members/{member_id}` | Yes | Update a member's role |
| `DELETE` | `/api/v1/workspaces/current/members/{member_id}` | Yes | Remove a member |
| `POST` | `/api/v1/workspaces/current/invitations` | Yes | Invite a user by email |
| `GET` | `/api/v1/workspaces/current/invitations` | Yes | List pending invitations |
| `DELETE` | `/api/v1/workspaces/current/invitations/{id}` | Yes | Cancel a pending invitation |
| `GET` | `/api/v1/workspaces/invitations` | No | List the current user's pending invitations |
| `POST` | `/api/v1/workspaces/invitations/accept` | No | Accept an invitation by token |
| `POST` | `/api/v1/workspaces/invitations/decline` | No | Decline an invitation by token |
| `POST` | `/api/v1/workspaces/invitations/{id}/accept` | No | Accept an invitation by ID |
| `POST` | `/api/v1/workspaces/invitations/{id}/decline` | No | Decline an invitation by ID |
| `GET` | `/api/v1/workspaces/current/plan` | Yes | Get the effective plan and limits |
| `GET` | `/api/v1/workspaces/current/settings` | Yes | Get operational settings |
| `PUT` | `/api/v1/workspaces/current/settings` | Yes | Update operational settings (admin+) |
| `GET` | `/api/v1/workspaces/current/audit-log` | Yes | Workspace audit trail (admin+) |

Member and invitation details are documented in [Members and Invitations](./members-and-invitations). Operational settings, plan, and the audit log are documented in [Settings, Plan, and Audit Log](./settings).

## Creating a workspace

```
POST /api/v1/workspaces
```

Request body:

```json
{
  "name": "Acme Inc",
  "slug": "acme",
  "description": "Marketing and transactional mail",
  "default_language": "en"
}
```

Only `name` is required. If `slug` is omitted it is derived from the name; slugs must contain only lowercase letters, numbers, and hyphens, and must be unique. `default_language` defaults to `en`. The caller becomes the workspace **owner**.

```bash
curl -X POST http://localhost:9000/api/v1/workspaces \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{"name": "Acme Inc", "slug": "acme"}'
```

Response (`201`):

```json
{
  "data": {
    "id": 1,
    "name": "Acme Inc",
    "slug": "acme",
    "description": "",
    "owner_id": 42,
    "role": "owner",
    "system": false,
    "created_at": "2026-05-31T10:00:00Z"
  }
}
```

Creating a workspace is subject to your plan's workspace quota; exceeding it returns `403`. Creating a duplicate slug returns `409`.

## Listing workspaces

```
GET /api/v1/workspaces
```

Returns every workspace the current user is a member of. Each entry carries the caller's `role` in that workspace and a `system` flag, which is true only for the built-in platform workspace.

## Get, update, and delete the current workspace

```
GET    /api/v1/workspaces/current
PUT    /api/v1/workspaces/current
DELETE /api/v1/workspaces/current
```

`PUT` accepts any subset of the following; empty fields are left unchanged:

```json
{
  "name": "Acme Corporation",
  "description": "Updated description",
  "default_language": "fr"
}
```

`DELETE` removes the workspace and returns `204`.

Two workspaces refuse deletion:

- The **system workspace** returns `409`. It is the built-in platform workspace, created on first boot, owned by the first administrator, and holds platform-managed resources. It cannot be renamed either, and only platform administrators are members.
- Your **last remaining workspace** returns `400`. Everything in Posta belongs to a workspace, so deleting the only one you belong to would leave you with nowhere to work. Create another first.

```bash
curl -X PUT http://localhost:9000/api/v1/workspaces/current \
  -H "Authorization: Bearer <jwt>" \
  -H "X-Posta-Workspace-Id: 1" \
  -H "Content-Type: application/json" \
  -d '{"description": "Updated description"}'
```
