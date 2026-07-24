---
sidebar_position: 1
title: Dashboard Stats
description: Dashboard statistics and metrics
---

# Dashboard Statistics

Get an overview of your email sending activity. All dashboard and email listing routes are workspace-scoped and require the `X-Posta-Workspace-Id` header.

## Workspace Dashboard Stats

```
GET /api/v1/workspaces/current/dashboard/stats
```

```bash
curl http://localhost:9000/api/v1/workspaces/current/dashboard/stats \
  -H "Authorization: Bearer <jwt>" \
  -H "X-Posta-Workspace-Id: 1"
```

Response:

```json
{
  "success": true,
  "data": {
    "total_emails": 15420,
    "queued_emails": 12,
    "processing_emails": 5,
    "sent_emails": 14890,
    "failed_emails": 230,
    "suppressed_emails": 40,
    "failure_rate": 1.49,
    "total_domains": 3,
    "total_smtp_servers": 2,
    "total_api_keys": 5,
    "active_api_keys": 4,
    "total_contacts": 8200,
    "total_bounces": 120,
    "total_suppressions": 40,
    "total_webhooks": 2,
    "total_inbound": 300,
    "forwarded_inbound": 280,
    "failed_inbound": 20,
    "daily_volume": [
      {"date": "2026-05-18", "sent": 800, "failed": 10},
      {"date": "2026-05-19", "sent": 950, "failed": 8}
    ],
    "webhook_deliveries": {
      "total": 500,
      "success": 490,
      "failed": 10
    }
  }
}
```

## Email Listing

View sent emails for the workspace with pagination:

```
GET /api/v1/workspaces/current/emails?page=1&size=20
```

The same endpoint also supports search, filtering, and sorting — see
[Email Log](/docs/email-sending/email-log).

## Email Details

Get full details for a specific email:

```
GET /api/v1/workspaces/current/emails/{id}
```

:::note
Email content may be redacted depending on privacy settings configured by the administrator.
:::
