---
sidebar_position: 3
title: Versioning
description: Manage template versions
---

# Template Versioning

Each template can have multiple versions. Only one version is active at a time — the active version is used when sending emails.

## List Versions

```
GET /api/v1/workspaces/current/templates/{templateId}/versions
```

## Create a New Version

```
POST /api/v1/workspaces/current/templates/{templateId}/versions
```

```json
{
  "html": "<h1>Welcome, {{name}}!</h1><p>New design with updated branding.</p>",
  "text": "Welcome, {{name}}! New design with updated branding.",
  "sample_data": {"name": "Alice"},
  "note": "Updated branding for Q2 campaign"
}
```

Response (`201`):

```json
{
  "success": true,
  "data": {
    "id": "version-uuid",
    "version_number": 2,
    "created_at": "2026-01-15T00:00:00Z"
  }
}
```

## Update a Version

```
PUT /api/v1/workspaces/current/templates/{templateId}/versions/{versionId}
```

```json
{
  "html": "<h1>Updated content</h1>",
  "text": "Updated content",
  "sample_data": {"name": "Alice"}
}
```

## Activate a Version

Switch the active version used for sending:

```
POST /api/v1/workspaces/current/templates/{templateId}/activate/{versionId}
```

The response includes the updated template with the new `active_version_id`.

## Delete a Version

```
DELETE /api/v1/workspaces/current/templates/{templateId}/versions/{versionId}
```

Returns `204 No Content`.

:::caution
You cannot delete the currently active version. Activate a different version first.
:::

## Workflow

A typical versioning workflow:

1. Create a new version with updated content
2. Preview it with sample data
3. Send a test email to verify rendering
4. Activate the new version
5. All subsequent sends use the new version
