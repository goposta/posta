---
sidebar_position: 2
title: Platform Settings
description: Configure platform-wide settings
---

# Platform Settings

Administrators can configure platform-wide settings that affect all users via the dashboard or `GET/PUT /api/v1/admin/settings`.

:::tip
For the full request/response schema, see the interactive [API Reference](https://app.goposta.dev/docs).
:::

## Available Settings

| Key | Type | Description |
|-----|------|-------------|
| `registration_enabled` | boolean | Allow new user registration |
| `email_retention_days` | integer | Days to retain email logs |
| `webhook_retention_days` | integer | Days to retain webhook delivery history |
| `audit_log_retention_days` | integer | Days to retain audit log entries |
| `bounce_handling` | string | Bounce handling configuration |
| `email_content_visible` | boolean | Show email content in logs/details |

Settings are stored as key-value pairs and can be updated in bulk.
