---
sidebar_position: 2
title: Email Analytics
description: Detailed email analytics and trends
---

# Email Analytics

Posta provides detailed analytics on email delivery performance with date range filtering. All workspace analytics routes require the `X-Posta-Workspace-Id` header alongside a JWT bearer token.

## Daily Analytics

```
GET /api/v1/workspaces/current/analytics?from=2026-01-01&to=2026-01-31&status=sent
```

Query parameters:

| Parameter | Type | Description |
|-----------|------|-------------|
| `from` | date (YYYY-MM-DD) | Start of date range (default: 30 days ago) |
| `to` | date (YYYY-MM-DD) | End of date range (default: today) |
| `status` | string | Filter by status: `sent`, `failed`, `queued`, etc. |

```bash
curl "http://localhost:9000/api/v1/workspaces/current/analytics?from=2026-01-01&to=2026-01-31" \
  -H "Authorization: Bearer <jwt>" \
  -H "X-Posta-Workspace-Id: 1"
```

Response:

```json
{
  "success": true,
  "data": {
    "daily_counts": [
      {"date": "2026-01-01", "count": 153},
      {"date": "2026-01-02", "count": 205}
    ],
    "status_breakdown": [
      {"status": "sent", "count": 14890},
      {"status": "failed", "count": 230},
      {"status": "queued", "count": 12}
    ]
  }
}
```

## Dashboard Analytics

```
GET /api/v1/workspaces/current/analytics/dashboard?from=2026-01-01&to=2026-01-31
```

Provides:

- **Delivery rate trends** — Daily sent/failed counts and delivery rate percentage per day
- **Bounce rate trends** — Daily hard, soft, and complaint bounce counts
- **Latency percentiles** — p50, p75, p90, p99 and average email delivery latency (seconds)

```bash
curl "http://localhost:9000/api/v1/workspaces/current/analytics/dashboard?from=2026-01-01&to=2026-01-31" \
  -H "Authorization: Bearer <jwt>" \
  -H "X-Posta-Workspace-Id: 1"
```

Response:

```json
{
  "success": true,
  "data": {
    "delivery_rate_trends": [
      {"date": "2026-01-01", "sent": 150, "failed": 3, "total": 153, "delivery_rate": 98.04}
    ],
    "bounce_rate_trends": [
      {"date": "2026-01-01", "hard": 1, "soft": 2, "complaint": 0, "total": 3}
    ],
    "latency_percentiles": {
      "p50": 1.2,
      "p75": 2.1,
      "p90": 4.5,
      "p99": 12.3,
      "avg": 1.8
    }
  }
}
```

## Provider Breakdown

Returns sent/failed counts and delivery rate grouped by recipient mailbox provider (Gmail, Outlook, Yahoo, and others):

```
GET /api/v1/workspaces/current/analytics/providers?from=2026-01-01&to=2026-01-31
```

```bash
curl "http://localhost:9000/api/v1/workspaces/current/analytics/providers?from=2026-01-01&to=2026-01-31" \
  -H "Authorization: Bearer <jwt>" \
  -H "X-Posta-Workspace-Id: 1"
```

Response:

```json
{
  "success": true,
  "data": {
    "providers": [
      {
        "provider": "Gmail",
        "sent": 8200,
        "failed": 80,
        "bounced": 15,
        "total": 8295,
        "delivery_rate": 99.03
      },
      {
        "provider": "Outlook",
        "sent": 3100,
        "failed": 45,
        "bounced": 8,
        "total": 3153,
        "delivery_rate": 98.57
      },
      {
        "provider": "Other",
        "sent": 3590,
        "failed": 105,
        "bounced": 17,
        "total": 3712,
        "delivery_rate": 97.15
      }
    ]
  }
}
```

Providers are sorted by total volume, descending. The `bounced` field counts suppressed emails for that provider. Results cover the requested date range and default to the last 30 days when no range is supplied.

## Admin Platform Analytics

Administrators can view platform-wide analytics:

```
GET /api/v1/admin/analytics?from=2026-01-01&to=2026-01-31
```

```
GET /api/v1/admin/analytics/dashboard?from=2026-01-01&to=2026-01-31
```

```
GET /api/v1/admin/analytics/providers?from=2026-01-01&to=2026-01-31
```

These endpoints aggregate data across all workspaces and users.
