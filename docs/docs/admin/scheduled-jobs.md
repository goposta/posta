---
sidebar_position: 5
title: Scheduled Jobs
description: View and manage scheduled background jobs
---

# Scheduled Jobs

Posta runs scheduled background jobs for maintenance tasks. All times are UTC.

## List Jobs

```
GET /api/v1/admin/jobs
```

Response:

```json
{
  "success": true,
  "data": [
    {
      "name": "account-cleanup",
      "schedule": "0 2 * * *",
      "running": false,
      "last_run_at": "2026-01-15T02:00:00Z",
      "last_error": "",
      "next_run_at": "2026-01-16T02:00:00Z"
    },
    {
      "name": "retention-cleanup",
      "schedule": "0 3 * * *",
      "running": false,
      "last_run_at": "2026-01-15T03:00:00Z",
      "last_error": "",
      "next_run_at": "2026-01-16T03:00:00Z"
    }
  ]
}
```

Each entry includes:

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Job identifier |
| `schedule` | string | Cron expression |
| `running` | boolean | Whether the job is currently executing |
| `last_run_at` | string\|null | ISO-8601 timestamp of last execution |
| `last_error` | string | Last error message, empty if last run succeeded |
| `next_run_at` | string\|null | ISO-8601 timestamp of next scheduled run |

## Built-in Jobs

| Job | Schedule | Description |
|-----|----------|-------------|
| `account-cleanup` | Daily at 02:00 UTC | Permanently deletes user accounts whose scheduled deletion date has passed |
| `retention-cleanup` | Daily at 03:00 UTC | Purges expired email logs, inbound emails, webhook deliveries, audit events, and tracking data based on platform retention settings |
| `daily-report` | Daily at 07:00 UTC | Enqueues per-workspace daily summary report emails for workspace owners and admins |
| `api-key-expiry` | Daily at 08:00 UTC | Notifies workspace admins of API keys expiring within 7 days |
| `bounce-alert` | Daily at 09:00 UTC | Alerts workspace admins when their bounce rate exceeds 5% over the past 24 hours |
| `campaign-restart` | Every 5 minutes | Re-enqueues batches for campaigns that have been stuck in `sending` status for more than 10 minutes |

:::note
The cron manager is only active when the Asynq Redis queue is configured. If the queue is unavailable, no jobs run and `GET /api/v1/admin/jobs` returns an empty list.
:::
