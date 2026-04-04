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

## Creating a List

```
POST /api/v1/users/me/subscriber-lists
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
GET /api/v1/users/me/subscriber-lists?page=0&size=20
```

Response includes `member_count` for each list.

## Getting a List

```
GET /api/v1/users/me/subscriber-lists/{id}
```

## Updating a List

```
PUT /api/v1/users/me/subscriber-lists/{id}
```

All fields are optional.

## Deleting a List

```
DELETE /api/v1/users/me/subscriber-lists/{id}
```

## Managing Members (Static Lists)

### Add a Subscriber

```
POST /api/v1/users/me/subscriber-lists/{id}/members
```

```json
{
  "subscriber_id": 42
}
```

Only works for static lists. Adding a subscriber to a dynamic list returns an error.

### Remove a Subscriber

```
DELETE /api/v1/users/me/subscriber-lists/{id}/members
```

```json
{
  "subscriber_id": 42
}
```

### List Members

```
GET /api/v1/users/me/subscriber-lists/{id}/members?page=0&size=20
```

## Preview Segment

Test filter rules before creating a dynamic list:

```
POST /api/v1/users/me/subscriber-lists/preview-segment
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
