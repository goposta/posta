---
sidebar_position: 1
title: User Management
description: Manage users from the admin panel
---

# User Management

Administrators can create, update, and manage user accounts across the platform.

## Create a User

```
POST /api/v1/admin/users
```

```json
{
  "name": "New User",
  "email": "user@example.com",
  "password": "secure-password",
  "role": "user"
}
```

| Role | Description |
|------|-------------|
| `user` | Standard user — can send emails, manage own resources |
| `admin` | Administrator — full platform access |

Returns `409 Conflict` if the email already exists. A personal workspace is automatically provisioned for the new user.

## List Users

```
GET /api/v1/admin/users?page=1&size=20
```

## Update a User

```
PUT /api/v1/admin/users/{id}
```

```json
{
  "role": "admin",
  "active": true,
  "email_verified": true
}
```

All fields are optional. Setting `active` to `false` disables the account. You cannot disable your own account.

## User Metrics

Get sending statistics for a specific user:

```
GET /api/v1/admin/users/{id}/metrics
```

```json
{
  "success": true,
  "data": {
    "user": { ... },
    "total_emails": 5000,
    "sent_emails": 4850,
    "failed_emails": 100,
    "suppressed_emails": 50,
    "failure_rate": 2.0,
    "total_api_keys": 5,
    "active_api_keys": 3,
    "total_contacts": 1200,
    "total_bounces": 30,
    "total_suppressions": 50,
    "total_domains": 2,
    "total_smtp_servers": 1,
    "total_inbound": 100,
    "forwarded_inbound": 80,
    "failed_inbound": 5,
    "webhook_deliveries": { ... }
  }
}
```

## List User Workspaces

```
GET /api/v1/admin/users/{id}/workspaces
```

Returns all workspaces a user belongs to, including their plan information.

## Delete a User

```
DELETE /api/v1/admin/users/{id}
```

Disables the account and schedules it for permanent deletion after 7 days. Returns `400 Bad Request` if deletion is already scheduled.

## Force Delete a User

```
DELETE /api/v1/admin/users/{id}/force
```

Permanently deletes a user and all their data immediately. The user must be disabled (`active: false`) before this endpoint can be called.

:::caution
Force deletion is irreversible. All workspaces, emails, templates, and subscriber data owned by the user are permanently removed.
:::

## Cancel a Scheduled Deletion

```
POST /api/v1/admin/users/{id}/cancel-deletion
```

Cancels a pending account deletion and re-enables the user.

## Disable 2FA for a User

If a user loses access to their authenticator:

```
DELETE /api/v1/admin/users/{id}/2fa
```

## Revoke All Sessions

Force a user to re-authenticate by revoking all their active sessions:

```
POST /api/v1/admin/users/{id}/revoke-sessions
```

## User Plan

### Assign a Plan

```
POST /api/v1/admin/users/{id}/plan
```

```json
{
  "plan_id": 2
}
```

### Get Effective Plan

```
GET /api/v1/admin/users/{id}/plan
```
