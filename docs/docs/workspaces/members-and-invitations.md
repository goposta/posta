---
sidebar_position: 2
title: Members and Invitations
description: Manage workspace members, roles, and invitations
---

# Members and Invitations

Workspaces are collaborative. Owners and admins add people by sending email invitations; once accepted, the invitee becomes a member with the role chosen at invitation time. All endpoints here use JWT bearer authentication. The `/workspaces/current/*` routes additionally require the `X-Posta-Workspace-Id` header.

## Members

### List members

```
GET /api/v1/workspaces/current/members
```

```bash
curl -X GET http://localhost:9000/api/v1/workspaces/current/members \
  -H "Authorization: Bearer <jwt>" \
  -H "X-Posta-Workspace-Id: 1"
```

Response:

```json
{
  "data": [
    {
      "id": 5,
      "user_id": 42,
      "name": "Ada Lovelace",
      "email": "ada@example.com",
      "role": "owner",
      "created_at": "2026-05-31T10:00:00Z"
    }
  ]
}
```

### Update a member's role

```
PUT /api/v1/workspaces/current/members/{member_id}
```

`member_id` is the **user ID** of the member (the `user_id` field returned by the list endpoint).

```json
{
  "role": "editor"
}
```

Valid roles are `admin`, `editor`, and `viewer`. The `owner` role cannot be assigned (`400`), and an existing owner's role cannot be changed (`400`). The member receives a role-change notification email. Returns a message payload on success.

### Remove a member

```
DELETE /api/v1/workspaces/current/members/{member_id}
```

Removes the member identified by their user ID and returns `204`. The workspace owner cannot be removed (`400`).

## Invitations (workspace side)

Owners and admins manage outgoing invitations under `/workspaces/current/invitations`.

### Invite a member

```
POST /api/v1/workspaces/current/invitations
```

```json
{
  "email": "newuser@example.com",
  "role": "editor"
}
```

Both fields are required. `role` may be `admin`, `editor`, or `viewer` — inviting as `owner` is rejected (`400`). If the email already belongs to a member, the request returns `409`. The inviter's email must be verified.

A pending invitation is created with a unique token, valid for **7 days**, and an invitation email is sent to the address with an accept link of the form `/<app>/invitations?token=<token>`.

Response (`201`):

```json
{
  "data": {
    "id": 12,
    "workspace_id": 1,
    "workspace": "",
    "email": "newuser@example.com",
    "role": "editor",
    "status": "pending",
    "expires_at": "2026-06-07T10:00:00Z",
    "created_at": "2026-05-31T10:00:00Z"
  }
}
```

```bash
curl -X POST http://localhost:9000/api/v1/workspaces/current/invitations \
  -H "Authorization: Bearer <jwt>" \
  -H "X-Posta-Workspace-Id: 1" \
  -H "Content-Type: application/json" \
  -d '{"email": "newuser@example.com", "role": "editor"}'
```

### List pending invitations

```
GET /api/v1/workspaces/current/invitations
```

Returns the workspace's pending invitations as an array of the object shown above.

### Cancel an invitation

```
DELETE /api/v1/workspaces/current/invitations/{id}
```

Deletes the pending invitation by its ID and returns `204`.

## Invitations (invitee side)

These routes are for the **recipient** of an invitation. They use JWT auth only and do **not** require the workspace header — the invitee is not yet a member.

### List my invitations

```
GET /api/v1/invitations
```

Lists all pending invitations addressed to the authenticated user's email. Each entry includes the originating workspace name in the `workspace` field:

```json
{
  "data": [
    {
      "id": 12,
      "workspace_id": 1,
      "workspace": "Acme Inc",
      "email": "newuser@example.com",
      "role": "editor",
      "status": "pending",
      "expires_at": "2026-06-07T10:00:00Z",
      "created_at": "2026-05-31T10:00:00Z"
    }
  ]
}
```

### Accept or decline by token

Used by the link in the invitation email.

```
POST /api/v1/invitations/accept
POST /api/v1/invitations/decline
```

```json
{
  "token": "<invitation_token>"
}
```

On accept, the invitee is added to the workspace with the invited role and the invitation is marked `accepted`. The invitation's email must match the authenticated user (`403` otherwise). Accepting a non-pending or expired invitation returns `400`; if the user is already a member the invitation is marked accepted and `409` is returned.

A successful accept returns the joined workspace:

```json
{
  "data": {
    "message": "joined workspace \"Acme Inc\"",
    "workspace_id": 1
  }
}
```

### Accept or decline by ID

Used by the dashboard when the user is viewing their pending invitations.

```
POST /api/v1/invitations/{id}/accept
POST /api/v1/invitations/{id}/decline
```

These take no body. The same validation applies: the invitation must be pending, not expired, and addressed to the authenticated user's email.

```bash
curl -X POST http://localhost:9000/api/v1/invitations/12/accept \
  -H "Authorization: Bearer <jwt>"
```
