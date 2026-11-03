---
sidebar_position: 2
title: Creating Templates
description: Create and manage email templates
---

# Creating Templates

## Names

A template has two names:

- **`name`**: its identifier, like `welcome-email`. It is what you send as `template` and is unique in the workspace: up to 63 lowercase letters, digits and hyphens.
- **`display_name`**: its label in the dashboard, like `Welcome Email`.

When you create a template you can send either. A missing `name` is derived from the display name, and a missing display name is the name.

:::caution Changed in 0.18
Before 0.18 a template had only `name`, which was both its label and what you sent. Upgrading keeps every label as the `display_name` and gives each template a `name` derived from it: `Welcome Email` becomes `welcome-email`. Sending by the old label still works until 0.19 and logs a deprecation warning; switch to the new names. Stylesheets, forms, workspaces and OAuth providers follow the same scheme.
:::

## Create a Template

```
POST /api/v1/workspaces/current/templates
```

**Authentication:** JWT token + `X-Posta-Workspace-Id` header

```bash
curl -X POST http://localhost:9000/api/v1/workspaces/current/templates \
  -H "Authorization: Bearer <jwt-token>" \
  -H "X-Posta-Workspace-Id: 1" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "welcome",
    "display_name": "Welcome",
    "description": "Welcome email for new users",
    "html": "<h1>Welcome, {{name}}!</h1><p>Thanks for joining.</p>",
    "text": "Welcome, {{name}}! Thanks for joining.",
    "sample_data": {
      "name": "Alice"
    }
  }'
```

Response:

```json
{
  "success": true,
  "data": {
    "id": "uuid",
    "name": "welcome",
    "display_name": "Welcome",
    "description": "Welcome email for new users",
    "active_version_id": "version-uuid",
    "created_at": "2026-01-01T00:00:00Z"
  }
}
```

:::info
Template names must be unique. A `409 Conflict` is returned if the name already exists.
:::

## List Templates

```
GET /api/v1/workspaces/current/templates?page=1&size=20
```

## Update a Template

```
PUT /api/v1/workspaces/current/templates/{id}
```

```json
{
  "display_name": "Welcome (v2)",
  "description": "Updated welcome email",
  "default_language": "en"
}
```

Changing `name` renames the template for sending: callers that send the old name stop finding it.

## Delete a Template

```
DELETE /api/v1/workspaces/current/templates/{id}
```

Returns `204 No Content`.

## Using Templates for Sending

Once created, reference templates by name in the send API:

```bash
curl -X POST http://localhost:9000/api/v1/emails/send-template \
  -H "Authorization: Bearer <api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "template": "welcome",
    "to": ["user@example.com"],
    "template_data": {
      "name": "Bob"
    }
  }'
```
