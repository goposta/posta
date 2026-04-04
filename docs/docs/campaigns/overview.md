---
sidebar_position: 1
title: Overview
description: Create and send email campaigns to subscriber lists
---

# Campaigns

Campaigns let you send template-based emails to subscriber lists. They support scheduling, send rate control, A/B testing, and lifecycle management (pause, resume, cancel).

:::tip
For the full request/response schema, see the interactive [API Reference](https://app.goposta.dev/docs).
:::

## Creating a Campaign

```
POST /api/v1/users/me/campaigns
```

```json
{
  "name": "April Newsletter",
  "subject": "What's new in April",
  "from_email": "newsletter@example.com",
  "from_name": "Acme Team",
  "template_id": 5,
  "template_version_id": 12,
  "language": "en",
  "template_data": {
    "month": "April",
    "featured_url": "https://example.com/april"
  },
  "list_id": 3,
  "send_rate": 100,
  "scheduled_at": "2026-04-10T09:00:00Z"
}
```

### Required Fields

| Field | Description |
|-------|-------------|
| `name` | Campaign name (internal reference) |
| `subject` | Email subject line |
| `from_email` | Sender email address |
| `template_id` | Template to use for the email body |
| `list_id` | Subscriber list to send to |

### Optional Fields

| Field | Default | Description |
|-------|---------|-------------|
| `from_name` | — | Sender display name |
| `template_version_id` | — | Specific template version (uses active version if omitted) |
| `language` | `en` | Template localization language |
| `template_data` | — | Variables to pass to the template |
| `send_rate` | `0` | Max emails per minute (`0` = unlimited) |
| `send_at_local_time` | `false` | Send at scheduled time in each subscriber's local timezone |
| `ab_test_enabled` | `false` | Enable A/B testing |
| `ab_test_variants` | — | A/B test variant configuration |
| `scheduled_at` | — | Schedule for future delivery |

## Campaign Statuses

| Status | Description |
|--------|-------------|
| `draft` | Initial state, can be edited |
| `scheduled` | Scheduled for future delivery |
| `sending` | Currently sending to subscribers |
| `sent` | All messages delivered |
| `paused` | Sending paused, can be resumed |
| `cancelled` | Permanently cancelled |

## Listing Campaigns

```
GET /api/v1/users/me/campaigns?page=0&size=20
```

Optional query parameters:

| Parameter | Description |
|-----------|-------------|
| `page` | Page number (default: 0) |
| `size` | Page size (default: 20) |
| `status` | Filter by status |

Response includes delivery stats for each campaign:

```json
{
  "id": 1,
  "name": "April Newsletter",
  "status": "sending",
  "stats": {
    "total": 5000,
    "pending": 2100,
    "queued": 500,
    "sent": 2300,
    "failed": 50,
    "skipped": 50
  }
}
```

## Getting a Campaign

```
GET /api/v1/users/me/campaigns/{id}
```

## Updating a Campaign

```
PUT /api/v1/users/me/campaigns/{id}
```

Only `draft` campaigns can be updated. All fields are optional.

## Deleting a Campaign

```
DELETE /api/v1/users/me/campaigns/{id}
```
