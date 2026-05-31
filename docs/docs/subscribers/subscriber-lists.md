---
sidebar_position: 3
title: Subscriber Lists
description: Organize subscribers into static and dynamic lists
---

# Subscriber Lists

Subscriber lists group subscribers for use in campaigns. Lists can be **static** (manually managed) or **dynamic** (rule-based segments).

:::tip
For the full request/response schema, see the interactive [API Reference](https://app.goposta.dev/docs).
:::

:::note
List management endpoints are workspace-scoped and require a JWT bearer token and the `X-Posta-Workspace-Id` header. The public subscribe/unsubscribe/resubscribe endpoints use an API key instead — see [Public Subscribe Operations](#public-subscribe-operations-api-key).
:::

## Creating a List

```
POST /api/v1/workspaces/current/subscriber-lists
```

### Static List

```json
{
  "name": "Newsletter Subscribers",
  "description": "Users who opted in to the weekly newsletter",
  "type": "static"
}
```

### Dynamic List (Segment)

```json
{
  "name": "Pro Users in US",
  "description": "Subscribers with pro plan in US timezone",
  "type": "dynamic",
  "filter_rules": [
    {
      "field": "custom_fields.plan",
      "operator": "equals",
      "value": "pro"
    },
    {
      "field": "timezone",
      "operator": "contains",
      "value": "America"
    }
  ]
}
```

Dynamic lists automatically include all subscribers matching the filter rules. Membership is calculated at query time.

## Listing Lists

```
GET /api/v1/workspaces/current/subscriber-lists?page=0&size=20
```

Response includes `member_count` for each list.

## Getting a List

```
GET /api/v1/workspaces/current/subscriber-lists/{id}
```

## Updating a List

```
PUT /api/v1/workspaces/current/subscriber-lists/{id}
```

All fields are optional.

## Deleting a List

```
DELETE /api/v1/workspaces/current/subscriber-lists/{id}
```

## Managing Members (Static Lists)

### Add a Subscriber

```
POST /api/v1/workspaces/current/subscriber-lists/{id}/members
```

```json
{
  "subscriber_id": 42
}
```

Only works for static lists. Adding a subscriber to a dynamic list returns an error.

### Remove a Subscriber

```
DELETE /api/v1/workspaces/current/subscriber-lists/{id}/members
```

```json
{
  "subscriber_id": 42
}
```

### List Members

```
GET /api/v1/workspaces/current/subscriber-lists/{id}/members?page=0&size=20
```

## Preview Segment

Test filter rules before creating a dynamic list:

```
POST /api/v1/workspaces/current/subscriber-lists/preview-segment
```

```json
{
  "filter_rules": [
    {
      "field": "status",
      "operator": "equals",
      "value": "subscribed"
    },
    {
      "field": "language",
      "operator": "equals",
      "value": "en"
    }
  ]
}
```

Response:

```json
{
  "success": true,
  "data": {
    "count": 1284
  }
}
```

## Filter Rules

Each rule has three properties:

| Property | Description |
|----------|-------------|
| `field` | Subscriber field to match (e.g., `status`, `language`, `timezone`, `custom_fields.plan`) |
| `operator` | Comparison operator (`equals`, `contains`, etc.) |
| `value` | Value to compare against |

Filter rules are combined with AND logic — subscribers must match all rules to be included.

## Public Subscribe Operations (API Key)

These endpoints are authenticated with an API key (`Authorization: Bearer <api_key>`) and do **not** require the `X-Posta-Workspace-Id` header. They are intended for external integrations such as sign-up forms and automation tools.

### Subscribe

Adds an email to a named list, creating the list on first use. Any prior per-list opt-out for the same (list, email) is cleared.

```
POST /api/v1/subscriber-lists/subscribe
```

```json
{
  "email": "user@example.com",
  "name": "Jane Doe",
  "list": "Newsletter"
}
```

`email` and `list` are required. `name` is optional and is only applied when the subscriber is new.

Response:

```json
{
  "success": true,
  "data": {
    "list_id": 3,
    "subscriber_id": 42,
    "email": "user@example.com",
    "action": "subscribed",
    "list_created": false,
    "subscriber_created": true,
    "member_added": true
  }
}
```

### Unsubscribe

Opts an email out of a specific list without changing the subscriber's global status.

```
POST /api/v1/subscriber-lists/{id}/unsubscribe
```

```json
{
  "email": "user@example.com",
  "reason": "user_request"
}
```

`email` is required. `reason` is optional (defaults to `"api"`).

### Resubscribe

Reverses a list-scoped opt-out and re-adds the subscriber to the list (static lists only). Idempotent.

```
POST /api/v1/subscriber-lists/{id}/resubscribe
```

```json
{
  "email": "user@example.com"
}
```
