---
sidebar_position: 1
title: Overview
description: Official Posta SDKs
---

# Official SDKs

Posta provides official client libraries for three languages. All SDKs offer the same core functionality:

| Method | Description |
|--------|-------------|
| `SendEmail` | Send a single email with HTML/text, attachments, and headers |
| `SendTemplateEmail` | Send an email using a pre-defined template |
| `SendBatch` | Send batch emails to multiple recipients |
| `GetEmailStatus` | Check delivery status by email UUID |

## Available SDKs

| Language | Package | Min Version |
|----------|---------|-------------|
| [Go](/docs/sdks/go) | `github.com/goposta/posta-go` | Go 1.25+ |
| [PHP](/docs/sdks/php) | `goposta/posta-php` | PHP 8.1+ |
| [Java](/docs/sdks/java) | `com.github.goposta:posta-java` | Java 11+ |

## Error Handling

All SDKs provide typed API errors that include:

- **HTTP status code** — The response status (e.g., 400, 401, 429)
- **Error info** — Structured error details with `code` and `message` fields

## Response Envelope

All API responses use a standard envelope:

```json
{
  "success": true,
  "data": { ... }
}
```

Error responses:

```json
{
  "success": false,
  "error": {
    "code": "validation_error",
    "message": "Invalid email address",
    "error": "to[0]: invalid format"
  }
}
```
