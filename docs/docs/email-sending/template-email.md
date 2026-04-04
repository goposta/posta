---
sidebar_position: 2
title: Template Email
description: Send emails using pre-defined templates
---

# Template Email

Send emails using pre-defined templates with variable substitution and multi-language support.

:::tip
For the full request/response schema, see the interactive [API Reference](https://app.goposta.dev/docs).
:::

## Example

```bash
curl -X POST http://localhost:9000/api/v1/emails/send-template \
  -H "Authorization: Bearer your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "template": "welcome",
    "to": ["user@example.com"],
    "from": "noreply@example.com",
    "language": "en",
    "template_data": {
      "name": "Alice",
      "activation_url": "https://example.com/activate?token=abc123"
    }
  }'
```

## Template Variables

Templates use `{{variable_name}}` syntax for variable substitution. The `template_data` object provides the values:

**Template HTML:**
```html
<h1>Welcome, {{name}}!</h1>
<p>Click <a href="{{activation_url}}">here</a> to activate your account.</p>
```

**template_data:**
```json
{
  "name": "Alice",
  "activation_url": "https://example.com/activate?token=abc123"
}
```

## Language Fallback

When a `language` is specified:

1. Posta looks for a localization matching that language on the active template version
2. If not found, falls back to the template's default language
3. If no default language is set, uses the base template content

## Preview Before Sending

Preview the rendered template without sending:

```bash
curl -X POST http://localhost:9000/api/v1/emails/preview \
  -H "Authorization: Bearer your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "template": "welcome",
    "language": "en",
    "template_data": {"name": "Alice"}
  }'
```
