---
sidebar_position: 7
title: Plans & Quotas
description: Configure plans with rate limits and resource quotas
---

# Plans & Quotas

Plans define resource limits and rate quotas for workspaces. Administrators can create plans with different tiers and assign them to workspaces.

:::tip
For the full request/response schema, see the interactive [API Reference](https://app.goposta.dev/docs).
:::

## Creating a Plan

```
POST /api/v1/admin/plans
```

```json
{
  "name": "Pro",
  "description": "For growing teams",
  "daily_rate_limit": 10000,
  "hourly_rate_limit": 1000,
  "max_attachment_size_mb": 25,
  "max_batch_size": 500,
  "max_api_keys": 20,
  "max_domains": 10,
  "max_smtp_servers": 5,
  "max_workspaces": 3,
  "email_log_retention_days": 90
}
```

A value of `0` for any limit means **unlimited**.

### Plan Fields

| Field | Default | Description |
|-------|---------|-------------|
| `name` | — | Plan name (required, unique) |
| `description` | — | Plan description |
| `is_default` | `false` | Set as the default plan for new workspaces |
| `daily_rate_limit` | `0` | Max emails per day |
| `hourly_rate_limit` | `0` | Max emails per hour |
| `max_attachment_size_mb` | `0` | Max attachment size in MB |
| `max_batch_size` | `0` | Max recipients per batch email |
| `max_api_keys` | `0` | Max API keys per workspace |
| `max_domains` | `0` | Max verified domains per workspace |
| `max_smtp_servers` | `0` | Max SMTP servers per workspace |
| `max_workspaces` | `0` | Max workspaces per user |
| `email_log_retention_days` | `0` | Days to retain email logs |

## Listing Plans

```
GET /api/v1/admin/plans
```

## Getting a Plan

```
GET /api/v1/admin/plans/{id}
```

## Updating a Plan

```
PUT /api/v1/admin/plans/{id}
```

All fields are optional. Include `is_active` to enable or disable a plan.

## Deleting a Plan

```
DELETE /api/v1/admin/plans/{id}?force=false
```

| Parameter | Default | Description |
|-----------|---------|-------------|
| `force` | `false` | If `true`, unassigns the plan from all workspaces before deletion. If `false` and the plan is assigned to workspaces, returns `409 Conflict`. |

## Setting the Default Plan

```
PATCH /api/v1/admin/plans/{id}/default
```

The previous default plan (if any) is automatically unset.

## Assigning a Plan to a Workspace

```
POST /api/v1/admin/workspaces/{id}/plan
```

```json
{
  "plan_id": 2
}
```

## Viewing a Workspace's Plan

### Admin View

```
GET /api/v1/admin/workspaces/{id}/plan
```

### User View

```
GET /api/v1/workspaces/current/plan
```

Returns the effective plan and limits for the current workspace.
