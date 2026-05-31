---
sidebar_position: 4
title: Shared SMTP Servers
description: Manage the platform-wide shared SMTP server pool
---

# Shared SMTP Servers

Administrators manage a pool of shared SMTP servers (`/api/v1/admin/servers`) that workspaces can use for delivery without configuring their own SMTP credentials. The connection is validated automatically when a server is created or updated.

:::note
For the per-workspace SMTP servers that individual workspaces own and control, see [SMTP Servers](/docs/smtp-domains/smtp-servers).
:::

## Create a Server

```
POST /api/v1/admin/servers
```

```json
{
  "name": "Primary Relay",
  "host": "smtp.example.com",
  "port": 587,
  "username": "relay@example.com",
  "password": "secret",
  "encryption": "tls",
  "max_retries": 3,
  "allowed_domains": ["example.com", "company.org"],
  "security_mode": "permissive"
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Display name |
| `host` | Yes | SMTP hostname |
| `port` | Yes | SMTP port (typically 25, 465, or 587) |
| `username` | No | SMTP authentication username |
| `password` | No | SMTP authentication password (stored encrypted) |
| `encryption` | No | `none`, `tls`, or `starttls` (default: `none`) |
| `max_retries` | No | Max delivery retry attempts (default: `0`) |
| `allowed_domains` | No | Restrict use to specific sender domains |
| `security_mode` | No | `permissive` or `strict` (see below) |

### Security Modes

| Mode | Description |
|------|-------------|
| `permissive` | Any workspace whose sender domain matches `allowed_domains` can use this server |
| `strict` | The workspace must have verified ownership of the sender domain via TXT record |

The SMTP connection is tested immediately on creation. If the connection fails, the server is saved with `status: invalid` and a `validation_error` message.

Returns `201 Created` on success.

## List Servers

```
GET /api/v1/admin/servers?page=1&size=20
```

## Get a Server

```
GET /api/v1/admin/servers/{id}
```

:::caution
The `password` field is never included in API responses.
:::

## Update a Server

```
PUT /api/v1/admin/servers/{id}
```

All fields are optional. If `host`, `port`, `username`, `password`, or `encryption` are changed, the connection is re-validated automatically.

To change status directly, include `"status": "enabled"` or `"status": "disabled"` in the body. Setting `status: enabled` triggers a connection re-validation before enabling.

## Delete a Server

```
DELETE /api/v1/admin/servers/{id}
```

Returns `204 No Content`.

## Enable a Server

Re-validates the connection and sets the server status to `enabled` if successful:

```
POST /api/v1/admin/servers/{id}/enable
```

If the connection test fails, the server is set to `invalid` and the response still returns `200 OK` with the updated server object.

## Disable a Server

```
POST /api/v1/admin/servers/{id}/disable
```

Immediately stops the server from being used for new deliveries.

## Test a Server

Test the SMTP connection without changing the server status:

```
POST /api/v1/admin/servers/{id}/test
```

```json
{
  "success": true,
  "data": {
    "message": "connection successful",
    "status": "enabled",
    "validated_at": "2026-05-31T10:00:00Z"
  }
}
```

If the connection fails, `success` is `false` and `message` contains the error.
