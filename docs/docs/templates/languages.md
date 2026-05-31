---
sidebar_position: 5
title: Languages
description: Manage the languages used for template localizations
---

# Languages

Languages are a workspace-level resource that defines which locales your
templates can be localized into. Each [template localization](/docs/templates/localization)
references a language by its `code`, so you typically create the languages you
need before adding localized content.

## What a language record is

A language record has these fields:

| Field | Type | Description |
|---|---|---|
| `id` | integer | Auto-generated identifier. |
| `code` | string | The locale code used to reference the language, e.g. `en`, `fr`, `de` (max 10 chars). |
| `name` | string | Human-readable name, e.g. `English`, `French`. |
| `is_default` | boolean | Whether this is the workspace's default language. Only one language can be the default — setting a new default clears the previous one. |
| `created_at` | timestamp | When the language was created. |

All language routes are workspace-scoped: they require a JWT plus the
`X-Posta-Workspace-Id` header (or a workspace-scoped API key, in which case the
header is implied).

## Create a Language

```
POST /api/v1/workspaces/current/languages
```

```json
{
  "code": "fr",
  "name": "French",
  "is_default": false
}
```

`code` and `name` are required. Set `is_default` to `true` to make this the
workspace default; any existing default is cleared automatically.

```bash
curl -X POST http://localhost:9000/api/v1/workspaces/current/languages \
  -H "Authorization: Bearer <jwt>" \
  -H "X-Posta-Workspace-Id: 1" \
  -H "Content-Type: application/json" \
  -d '{
    "code": "fr",
    "name": "French",
    "is_default": false
  }'
```

Response (`201`):

```json
{
  "success": true,
  "data": {
    "id": 1,
    "code": "fr",
    "name": "French",
    "is_default": false,
    "created_at": "2026-01-01T00:00:00Z"
  }
}
```

:::info
A `409 Conflict` is returned if the language code already exists in the workspace.
:::

## List Languages

```
GET /api/v1/workspaces/current/languages?page=1&size=20
```

Returns a paginated list of the workspace's languages.

## Update a Language

```
PUT /api/v1/workspaces/current/languages/{id}
```

```json
{
  "name": "Français",
  "is_default": true
}
```

All fields are optional. Provide only the fields you want to change. Setting
`is_default` to `true` clears the previous default language.

## Delete a Language

```
DELETE /api/v1/workspaces/current/languages/{id}
```

Returns `204 No Content`.

## Relationship to localizations

A language defines a locale at the workspace level. Template content is then
localized per template version: each localization stores `subject_template`,
`html_template`, and `text_template` for a given `language` code. See
[Localization](/docs/templates/localization) for adding and managing localized
template content, and how a language is resolved at send time.
