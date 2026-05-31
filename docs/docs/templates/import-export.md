---
sidebar_position: 7
title: Import & Export
description: Move templates between environments
---

# Import & Export

Export templates as JSON and import them into other Posta instances. This is useful for migrating templates between staging and production environments.

## Export a Template

```
GET /api/v1/workspaces/current/templates/{templateId}/export
```

Returns the complete template including all versions and localizations as a JSON file.

```json
{
  "name": "welcome",
  "description": "Welcome email for new users",
  "default_language": "en",
  "versions": [
    {
      "version_number": 1,
      "html": "<h1>Welcome, {{name}}!</h1>",
      "text": "Welcome, {{name}}!",
      "sample_data": {"name": "Alice"},
      "localizations": [
        {
          "language": "fr",
          "html": "<h1>Bienvenue, {{name}} !</h1>",
          "text": "Bienvenue, {{name}} !"
        }
      ]
    }
  ]
}
```

## Import a Template

```
POST /api/v1/workspaces/current/templates/import
```

Send the exported JSON as the request body. Returns `201 Created` with the new template.

```bash
curl -X POST http://localhost:9000/api/v1/workspaces/current/templates/import \
  -H "Authorization: Bearer <jwt-token>" \
  -H "X-Posta-Workspace-Id: 1" \
  -H "Content-Type: application/json" \
  -d @template-export.json
```

:::note
If a template with the same name already exists, the import will fail with `409 Conflict`. Rename the template in the JSON before importing, or delete the existing template first.
:::
