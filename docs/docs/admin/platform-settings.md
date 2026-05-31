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

## Get Settings

```
GET /api/v1/admin/settings
```

Returns all platform settings as a list of key-value entries.

## Update Settings

Settings are updated in bulk by supplying an array of key-value pairs:

```
PUT /api/v1/admin/settings
```

```json
{
  "settings": [
    { "key": "registration_enabled", "value": "true", "type": "bool" },
    { "key": "retention_days", "value": "60", "type": "int" }
  ]
}
```

Keys prefixed with `app.` are reserved and cannot be modified.

## Available Settings

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `registration_enabled` | bool | `false` | Allow new user self-registration |
| `require_email_verification` | bool | `true` | Require email verification on sign-up |
| `require_domain_verification` | bool | `true` | Require domain ownership verification before sending |
| `default_rate_limit_hourly` | int | `100` | Default hourly send limit for new workspaces |
| `default_rate_limit_daily` | int | `1000` | Default daily send limit for new workspaces |
| `max_batch_size` | int | `100` | Default max recipients per batch send |
| `max_attachment_size_mb` | int | `10` | Default max attachment size in MB |
| `retention_days` | int | `30` | Days to retain email logs |
| `audit_log_retention_days` | int | `90` | Days to retain audit log entries |
| `webhook_delivery_retention_days` | int | `30` | Days to retain webhook delivery history |
| `global_bounce_threshold` | int | `5` | Platform-wide bounce rate threshold (percent) |
| `smtp_timeout_seconds` | int | `30` | SMTP connection timeout in seconds |
| `maintenance_mode` | bool | `false` | Put the platform in maintenance mode |
| `allowed_signup_domains` | string | `""` | Comma-separated list of allowed sign-up email domains (empty = all) |
| `two_factor_required` | bool | `false` | Require 2FA for all users |
| `login_rate_limit_count` | int | `10` | Max login attempts per window |
| `login_rate_limit_window_minutes` | int | `15` | Rate limit window duration in minutes |
| `email_content_visibility` | bool | `false` | Show full email content (body/HTML) in logs and detail views |
| `custom_headers_enabled` | bool | `false` | Allow workspaces to add custom email headers |
