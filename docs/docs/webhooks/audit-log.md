---
sidebar_position: 4
title: Audit Log
description: Track workspace operational and personal security events
---

# Audit Log

Posta maintains two separate audit trails depending on what you need to audit.

## Workspace Operational Audit Log

The workspace audit log records operational activity scoped to a workspace: API key creation/revocation, template changes, SMTP server changes, webhook changes, and similar configuration actions. Requires the `admin` or `owner` workspace role.

```
GET /api/v1/workspaces/current/audit-log?page=1&size=20
```

```bash
curl "http://localhost:9000/api/v1/workspaces/current/audit-log?page=1&size=20" \
  -H "Authorization: Bearer <jwt>" \
  -H "X-Posta-Workspace-Id: 1"
```

### Query Parameters

| Parameter | Description |
|-----------|-------------|
| `page` | Page number (default: 0) |
| `size` | Items per page (default: 20) |

Response:

```json
{
  "success": true,
  "data": [
    {
      "id": 42,
      "category": "audit",
      "type": "apikey.created",
      "workspace_id": 1,
      "actor_id": 1,
      "actor_name": "admin@example.com",
      "client_ip": "203.0.113.42",
      "message": "API key created: My Key",
      "created_at": "2026-01-01T00:00:01Z"
    },
    {
      "id": 41,
      "category": "audit",
      "type": "webhook.created",
      "workspace_id": 1,
      "actor_id": 1,
      "actor_name": "admin@example.com",
      "client_ip": "203.0.113.42",
      "message": "Webhook created: https://your-app.com/webhooks/posta",
      "created_at": "2026-01-01T00:00:00Z"
    }
  ]
}
```

## Personal Security Audit Log

The personal audit log records security-sensitive events for the authenticated user's account: logins, password changes, 2FA changes, and session activity. This log is not workspace-scoped.

```
GET /api/v1/users/me/audit-log?page=1&size=20
```

```bash
curl "http://localhost:9000/api/v1/users/me/audit-log?page=1&size=20" \
  -H "Authorization: Bearer <jwt>"
```

Example entries:

```json
{
  "success": true,
  "data": [
    {
      "id": 10,
      "category": "audit",
      "type": "user.login",
      "actor_name": "admin@example.com",
      "client_ip": "203.0.113.42",
      "message": "User logged in",
      "created_at": "2026-01-01T00:00:00Z"
    }
  ]
}
```

## Summary

| Log | Endpoint | Scope | Typical entries |
|-----|----------|-------|-----------------|
| Workspace operational | `GET /api/v1/workspaces/current/audit-log` | Workspace (admin/owner) | API keys, webhooks, SMTP, templates |
| Personal security | `GET /api/v1/users/me/audit-log` | Authenticated user | Login, password, 2FA, sessions |

## Admin: Platform-Wide Events

Administrators can view all events across all users:

```
GET /api/v1/admin/events?page=1&size=20&category=email
```

### Real-Time Event Stream (SSE)

Subscribe to real-time events via Server-Sent Events:

```
GET /api/v1/admin/events/stream?token=<jwt-token>
```

```javascript
const eventSource = new EventSource(
  'http://localhost:9000/api/v1/admin/events/stream?token=your-jwt'
);

eventSource.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log('Event:', data);
};
```
