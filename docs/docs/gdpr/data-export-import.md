---
sidebar_position: 1
title: Data Export & Import
description: Export and import workspace data
---

# Data Export & Import

Posta supports full workspace data export and import for GDPR compliance and environment migration.

## Export All Workspace Data

```
GET /api/v1/workspaces/current/data/export
```

Returns all workspace data as JSON:

```bash
curl -X GET http://localhost:9000/api/v1/workspaces/current/data/export \
  -H "Authorization: Bearer <jwt>" \
  -H "X-Posta-Workspace-Id: 1"
```

```json
{
  "success": true,
  "data": {
    "posta_version": "1.0.0",
    "exported_at": "2026-05-31T12:00:00Z",
    "workspace_settings": {
      "name": "My Workspace",
      "description": "...",
      "default_language": "en"
    },
    "templates": [...],
    "stylesheets": [...],
    "languages": [...],
    "contacts": [...],
    "contact_lists": [...],
    "suppressions": [...],
    "webhooks": [...],
    "smtp_servers": [...],
    "domains": [...],
    "subscribers": [...],
    "subscriber_lists": [...]
  }
}
```

## Import Workspace Data

```
POST /api/v1/workspaces/current/data/import
```

Send the exported JSON payload as the request body. Items are inserted; existing items are not overwritten.

```bash
curl -X POST http://localhost:9000/api/v1/workspaces/current/data/import \
  -H "Authorization: Bearer <jwt>" \
  -H "X-Posta-Workspace-Id: 1" \
  -H "Content-Type: application/json" \
  -d @export.json
```

Response:

```json
{
  "success": true,
  "data": {
    "message": "Workspace data imported successfully",
    "imported_count": 42
  }
}
```

## Use Cases

- **GDPR data portability** — Export all workspace data on request
- **Environment migration** — Move settings between staging and production
- **Backup** — Periodic data exports for disaster recovery
