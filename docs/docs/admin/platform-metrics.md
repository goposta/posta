---
sidebar_position: 3
title: Platform Metrics
description: Platform-wide metrics and analytics
---

# Platform Metrics

Administrators can view aggregated metrics and analytics across all workspaces.

## Overview Metrics

```
GET /api/v1/admin/metrics
```

```json
{
  "success": true,
  "data": {
    "total_users": 150,
    "total_workspaces": 200,
    "total_emails": 500000,
    "queued_emails": 120,
    "processing_emails": 40,
    "sent_emails": 485000,
    "failed_emails": 10000,
    "suppressed_emails": 800,
    "failure_rate": 2.0,
    "total_api_keys": 300,
    "active_api_keys": 250,
    "total_bounces": 3000,
    "total_suppressions": 1200,
    "active_workers": 4,
    "shared_smtp_servers": 3,
    "total_domains": 45,
    "total_inbound": 5000,
    "forwarded_inbound": 4800,
    "failed_inbound": 50,
    "received_inbound": 5000,
    "rejected_inbound": 150,
    "webhook_deliveries": { ... },
    "server_uptime_seconds": 86400,
    "current_goroutines": 128,
    "current_memory_usage": 52428800,
    "active_sessions": 35,
    "failed_logins_last_24h": 12,
    "two_factor_adoption_rate": 42.5,
    "two_factor_users": 64
  }
}
```

## Platform Analytics

Date-filtered daily email counts and status breakdown across all workspaces:

```
GET /api/v1/admin/analytics?start_date=2026-01-01&end_date=2026-01-31
```

## Advanced Dashboard

```
GET /api/v1/admin/analytics/dashboard?start_date=2026-01-01&end_date=2026-01-31
```

Returns delivery rate trends, bounce rate graphs, and latency percentiles.

## Provider Breakdown

```
GET /api/v1/admin/analytics/providers?start_date=2026-01-01&end_date=2026-01-31
```

Returns sent/failed counts and delivery rate grouped by recipient mailbox provider (Gmail, Outlook, etc.).

## Event Log

```
GET /api/v1/admin/events?page=1&size=20
```

Lists platform activity and system events. Supports optional category filtering via `?category=<category>`.

## Real-Time Monitoring

### Event Stream (SSE)

Stream platform events in real-time:

```
GET /api/v1/admin/events/stream?token=<jwt-token>
```

### Metrics Stream (SSE)

Monitor background worker activity and API server runtime stats:

```
GET /api/v1/admin/metrics/stream?token=<jwt-token>
```

Emits two event types:

- `worker.status` — real-time worker count and processing details.
- `system.status` — server uptime, current goroutine count, and heap memory usage.
