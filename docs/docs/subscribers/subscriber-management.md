---
sidebar_position: 1
title: Subscriber Management
description: Create and manage email subscribers
---

# Subscriber Management

Subscribers represent people who receive campaign emails. Each subscriber has an email, optional profile data, custom fields, and a status.

:::tip
For the full request/response schema, see the interactive [API Reference](https://app.goposta.dev/docs).
:::

## Creating a Subscriber

```
POST /api/v1/users/me/subscribers
```

```json
{
  "email": "user@example.com",
  "name": "Jane Doe",
  "status": "subscribed",
  "timezone": "America/New_York",
  "language": "en",
  "custom_fields": {
    "company": "Acme Inc",
    "plan": "pro"
  }
}
```

Only `email` is required. Status defaults to `subscribed`.

### Subscriber Statuses

| Status | Description |
|--------|-------------|
| `subscribed` | Active subscriber, receives campaigns |
| `unsubscribed` | Opted out, will not receive campaigns |
| `bounced` | Email address bounced |
| `complained` | Reported as spam |

## Listing Subscribers

```
GET /api/v1/users/me/subscribers?page=0&size=20
```

Optional query parameters:

| Parameter | Description |
|-----------|-------------|
| `page` | Page number (default: 0) |
| `size` | Page size (default: 20) |
| `search` | Search by email or name |
| `status` | Filter by status |

## Getting a Subscriber

```
GET /api/v1/users/me/subscribers/{id}
```

## Updating a Subscriber

```
PUT /api/v1/users/me/subscribers/{id}
```

```json
{
  "name": "Jane Smith",
  "custom_fields": {
    "company": "New Corp",
    "plan": "enterprise"
  }
}
```

All fields are optional. When status changes to `unsubscribed`, the `unsubscribed_at` timestamp is set automatically.

## Deleting a Subscriber

```
DELETE /api/v1/users/me/subscribers/{id}
```

## Custom Fields

Subscribers support arbitrary key-value custom fields stored as JSON. These can be used for:

- Personalization in campaign templates via `{{ .custom_fields.company }}`
- Segmentation in dynamic subscriber lists
- Filtering and search

## Bulk Import

See [Bulk Import](/docs/subscribers/bulk-import) for importing subscribers from JSON and CSV files.
