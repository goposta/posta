---
sidebar_position: 2
title: Bulk Import
description: Import subscribers from JSON and CSV files
---

# Bulk Import

Import subscribers in bulk using JSON or CSV formats.

:::tip
For the full request/response schema, see the interactive [API Reference](https://app.goposta.dev/docs).
:::

## JSON Import

```
POST /api/v1/users/me/subscribers/import/json
```

```json
{
  "subscribers": [
    {
      "email": "alice@example.com",
      "name": "Alice",
      "timezone": "Europe/London",
      "language": "en",
      "custom_fields": {
        "company": "Acme"
      }
    },
    {
      "email": "bob@example.com",
      "name": "Bob",
      "language": "fr"
    }
  ]
}
```

Response:

```json
{
  "success": true,
  "data": {
    "created": 2,
    "skipped": 0,
    "total": 2
  }
}
```

Duplicate emails (already existing) are skipped, not updated.

## CSV Import

```
POST /api/v1/users/me/subscribers/import/csv
Content-Type: multipart/form-data
```

| Field | Required | Description |
|-------|----------|-------------|
| `file` | Yes | CSV file |
| `column_mapping` | No | JSON string mapping column indexes to fields |

### Default Column Mapping

If no `column_mapping` is provided:
- Column 0 = `email`
- Column 1 = `name`

### Custom Column Mapping

Map CSV columns to subscriber fields using a JSON object where keys are column indexes and values are field names:

```json
{
  "0": "email",
  "1": "name",
  "2": "custom_fields.company",
  "3": "custom_fields.role",
  "4": "timezone",
  "5": "language"
}
```

Custom fields use dot notation: `custom_fields.field_name`.

### Example

```bash
curl -X POST http://localhost:9000/api/v1/users/me/subscribers/import/csv \
  -H "Authorization: Bearer <your-token>" \
  -F "file=@subscribers.csv" \
  -F 'column_mapping={"0":"email","1":"name","2":"custom_fields.company"}'
```
