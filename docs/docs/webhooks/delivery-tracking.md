---
sidebar_position: 3
title: Delivery Tracking
description: Track webhook delivery history
---

# Webhook Delivery Tracking

Monitor the delivery status of your webhook notifications.

## View Delivery History

Webhook deliveries are workspace-scoped:

```
GET /api/v1/workspaces/current/webhook-deliveries?page=1&size=20
```

```bash
curl http://localhost:9000/api/v1/workspaces/current/webhook-deliveries \
  -H "Authorization: Bearer <jwt>" \
  -H "X-Posta-Workspace-Id: 1"
```

Response:

```json
{
  "success": true,
  "data": [
    {
      "id": 1,
      "webhook_id": 3,
      "user_id": 1,
      "workspace_id": 1,
      "event": "email.sent",
      "status": "success",
      "http_status_code": 200,
      "error_message": "",
      "attempt": 1,
      "created_at": "2026-01-01T00:00:01Z"
    },
    {
      "id": 2,
      "webhook_id": 3,
      "user_id": 1,
      "workspace_id": 1,
      "event": "email.failed",
      "status": "failed",
      "http_status_code": 500,
      "error_message": "HTTP 500",
      "attempt": 3,
      "created_at": "2026-01-01T00:01:00Z"
    }
  ]
}
```

## Delivery Details

| Field | Description |
|-------|-------------|
| `webhook_id` | ID of the webhook that fired |
| `event` | Event type that triggered the delivery |
| `status` | Outcome: `success` or `failed` |
| `http_status_code` | HTTP status code returned by your endpoint |
| `error_message` | Error detail when delivery failed |
| `attempt` | Which retry attempt this record represents |
| `created_at` | When the delivery was attempted |

## Retry Policy

- **Max retries:** Configured via `POSTA_WEBHOOK_MAX_RETRIES` (default: 3)
- **Timeout:** Configured via `POSTA_WEBHOOK_TIMEOUT_SECS` (default: 10s)
- **Backoff:** Exponential backoff between retries
- **Success:** Any `2xx` status code is considered successful

## Data Retention

Webhook delivery history is retained according to the `webhook_retention_days` platform setting, configurable by administrators.
