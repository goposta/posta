---
sidebar_position: 3
title: Shared SMTP Pool
description: Admin-managed shared SMTP servers
---

# Shared SMTP Pool

Administrators can configure a shared pool of SMTP servers available to all users. This simplifies setup for teams and ensures consistent email delivery.

## Admin Endpoints

All shared server endpoints require admin JWT authentication.

### Create a Shared Server

```
POST /api/v1/admin/servers
```

```json
{
  "name": "Company Outbound",
  "host": "smtp.company.com",
  "port": 587,
  "username": "posta@company.com",
  "password": "server-password",
  "encryption": "starttls",
  "max_retries": 5,
  "allowed_domains": ["company.com", "company.org"],
  "security_mode": "permissive"
}
```

`name` is required. `security_mode` accepts `permissive` (default) or `strict`. In strict mode, the sender must have verified ownership of the sending domain before the shared server is available to them.

### List Shared Servers

```
GET /api/v1/admin/servers?page=1&size=20
```

### Enable / Disable a Server

```
POST /api/v1/admin/servers/{id}/enable
POST /api/v1/admin/servers/{id}/disable
```

### Test Connection

```
POST /api/v1/admin/servers/{id}/test
```

### Update a Server

```
PUT /api/v1/admin/servers/{id}
```

### Delete a Server

```
DELETE /api/v1/admin/servers/{id}
```

## How It Works

When a user sends an email:

1. Posta first checks the user's own SMTP servers
2. If no user server is available or configured, falls back to the shared pool
3. The shared pool respects `allowed_domains` restrictions
4. Disabled servers in the pool are skipped
