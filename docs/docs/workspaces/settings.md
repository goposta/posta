---
sidebar_position: 3
title: Settings, Plan, and Audit Log
description: Operational workspace settings, plan limits, and the workspace audit trail
---

# Settings, Plan, and Audit Log

Each workspace has its own operational settings, an effective plan with sending limits, and an audit trail. All endpoints use JWT bearer authentication and require the `X-Posta-Workspace-Id` header.

:::note
These are the **operational** workspace settings and audit log, distinct from the per-user `/api/v1/users/me/settings` (personal notification preferences) and `/api/v1/users/me/audit-log` (personal security events).
:::

## Operational settings

### Get settings

```
GET /api/v1/workspaces/current/settings
```

Any member may read the settings.

```bash
curl -X GET http://localhost:9000/api/v1/workspaces/current/settings \
  -H "Authorization: Bearer <jwt>" \
  -H "X-Posta-Workspace-Id: 1"
```

Response:

```json
{
  "data": {
    "id": 1,
    "workspace_id": 1,
    "timezone": "UTC",
    "default_sender_name": "",
    "default_sender_email": "",
    "webhook_retry_count": 3,
    "api_key_expiry_days": 90,
    "bounce_auto_suppress": true,
    "require_verified_domain": false,
    "created_at": "2026-05-31T10:00:00Z",
    "updated_at": "2026-05-31T10:00:00Z"
  }
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `timezone` | string | `UTC` | Display/reporting timezone for the workspace |
| `default_sender_name` | string | — | Default From name applied when none is given |
| `default_sender_email` | string | — | Default From address applied when none is given |
| `webhook_retry_count` | int | `3` | Number of delivery retries for outgoing webhooks |
| `api_key_expiry_days` | int | `90` | Default lifetime for newly created API keys |
| `bounce_auto_suppress` | bool | `true` | Auto-suppress recipients on hard bounce / complaint |
| `require_verified_domain` | bool | `false` | Require a verified sending domain before sending |

### Update settings

```
PUT /api/v1/workspaces/current/settings
```

Requires the **admin** level or higher (owner or admin). Every field is optional; only the fields you include are changed.

```json
{
  "timezone": "Europe/Paris",
  "default_sender_name": "Acme",
  "default_sender_email": "no-reply@acme.com",
  "webhook_retry_count": 5,
  "api_key_expiry_days": 30,
  "bounce_auto_suppress": true,
  "require_verified_domain": true
}
```

```bash
curl -X PUT http://localhost:9000/api/v1/workspaces/current/settings \
  -H "Authorization: Bearer <jwt>" \
  -H "X-Posta-Workspace-Id: 1" \
  -H "Content-Type: application/json" \
  -d '{"require_verified_domain": true}'
```

The full settings object is returned on success.

## Workspace plan

```
GET /api/v1/workspaces/current/plan
```

Returns the effective plan and limits for the active workspace. A workspace may have its own assigned plan; otherwise the default plan applies.

```json
{
  "data": {
    "id": 2,
    "name": "Pro",
    "description": "",
    "is_default": false,
    "is_active": true,
    "daily_rate_limit": 50000,
    "hourly_rate_limit": 5000,
    "max_attachment_size_mb": 25,
    "max_batch_size": 1000,
    "max_api_keys": 50,
    "max_domains": 25,
    "max_smtp_servers": 10,
    "max_workspaces": 10,
    "email_log_retention_days": 90,
    "created_at": "2026-01-01T00:00:00Z",
    "updated_at": "2026-01-01T00:00:00Z"
  }
}
```

A limit of `0` means unlimited. Plans themselves are managed by platform administrators; see the Admin Panel documentation.

## Workspace audit log

```
GET /api/v1/workspaces/current/audit-log
```

Returns the audit trail for the active workspace — events recorded with `category = audit` and scoped to this workspace. Requires the **admin** level or higher (owner or admin).

Query parameters:

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `page` | int | `0` | Zero-based page index |
| `size` | int | `20` | Page size |
| `category` | string | — | Optional event category filter |

```bash
curl -X GET "http://localhost:9000/api/v1/workspaces/current/audit-log?page=0&size=20" \
  -H "Authorization: Bearer <jwt>" \
  -H "X-Posta-Workspace-Id: 1"
```

Each event has the following shape:

```json
{
  "id": 101,
  "category": "audit",
  "type": "member.role_updated",
  "workspace_id": 1,
  "actor_id": 42,
  "actor_name": "Ada Lovelace",
  "client_ip": "203.0.113.10",
  "message": "Updated role for member 7 to editor",
  "metadata": "{}",
  "created_at": "2026-05-31T10:05:00Z"
}
```

The response is paginated; results are wrapped in the standard pageable envelope.
