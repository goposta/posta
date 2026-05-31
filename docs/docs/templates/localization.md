---
sidebar_position: 4
title: Localization
description: Multi-language template support
---

# Template Localization

Add localized versions of templates for different languages. Localizations are attached to specific template versions.

## Managing Languages

Before adding localizations, create the languages you need. Languages are a
workspace-level resource — see [Languages](/docs/templates/languages) for the
full CRUD reference. Each localization references a language by its `code`
(for example `fr`).

## Adding Localizations

### Add a Localization to a Version

```
POST /api/v1/workspaces/current/templates/{templateId}/versions/{versionId}/localizations
```

```json
{
  "language": "fr",
  "subject_template": "Bienvenue, {{name}} !",
  "html_template": "<h1>Bienvenue, {{name}} !</h1><p>Merci de nous avoir rejoints.</p>",
  "text_template": "Bienvenue, {{name}} ! Merci de nous avoir rejoints."
}
```

`language` and `subject_template` are required. `html_template` and
`text_template` are optional, as is `builder_json` (the visual builder layout).

Response (`201`):

```json
{
  "success": true,
  "data": {
    "id": 1,
    "language": "fr",
    "subject_template": "Bienvenue, {{name}} !",
    "html_template": "...",
    "text_template": "..."
  }
}
```

:::info
A `409 Conflict` is returned if the language already exists for this version.
:::

### List Localizations

```
GET /api/v1/workspaces/current/templates/{templateId}/versions/{versionId}/localizations
```

### Update a Localization

```
PUT /api/v1/workspaces/current/localizations/{localizationId}
```

```json
{
  "subject_template": "Bienvenue !",
  "html_template": "<h1>Updated French content</h1>",
  "text_template": "Updated French content"
}
```

### Delete a Localization

```
DELETE /api/v1/workspaces/current/localizations/{localizationId}
```

## Sending with Language

Specify the language when sending:

```bash
curl -X POST http://localhost:9000/api/v1/emails/send-template \
  -H "Authorization: Bearer <api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "template": "welcome",
    "to": ["user@example.fr"],
    "language": "fr",
    "template_data": {"name": "Marie"}
  }'
```

### Language Resolution

1. Look for a localization matching the requested language on the active version
2. If not found, fall back to the template's default language
3. If no default is set, use the base template content

### Per-Recipient Language in Batch

```json
{
  "template": "newsletter",
  "recipients": [
    {"email": "bob@example.com", "language": "en", "template_data": {"name": "Bob"}},
    {"email": "marie@example.fr", "language": "fr", "template_data": {"name": "Marie"}},
    {"email": "hans@example.de", "language": "de", "template_data": {"name": "Hans"}}
  ]
}
```

## Preview a Localized Version

```
POST /api/v1/workspaces/current/templates/{templateId}/versions/{versionId}/preview
```

```json
{
  "language": "fr",
  "template_data": {"name": "Marie"}
}
```
