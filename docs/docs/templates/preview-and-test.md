---
sidebar_position: 6
title: Preview & Test
description: Preview templates and send test emails
---

# Preview & Test

Preview rendered templates and send test emails before going live.

## Preview a Template

Render a template with sample data without sending:

```
POST /api/v1/workspaces/current/templates/preview
```

```json
{
  "template": "welcome",
  "language": "en",
  "template_data": {
    "name": "Alice",
    "activation_url": "https://example.com/activate"
  }
}
```

Response:

```json
{
  "success": true,
  "data": {
    "subject": "Welcome to Our Platform",
    "html": "<h1>Welcome, Alice!</h1><p>Click <a href=\"https://example.com/activate\">here</a>...</p>",
    "text": "Welcome, Alice! Click here to activate..."
  }
}
```

## Send a Test Email

Send a test email using a specific template:

```
POST /api/v1/workspaces/current/templates/{templateId}/send-test
```

```json
{
  "to": ["test@example.com"],
  "template_data": {
    "name": "Test User"
  }
}
```

Response:

```json
{
  "success": true,
  "data": {
    "id": "email-uuid",
    "status": "queued"
  }
}
```

## Preview via API Key

The preview endpoint is also available via API key authentication:

```
POST /api/v1/emails/preview
```

```bash
curl -X POST http://localhost:9000/api/v1/emails/preview \
  -H "Authorization: Bearer <api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "template": "welcome",
    "template_data": {"name": "Alice"}
  }'
```
