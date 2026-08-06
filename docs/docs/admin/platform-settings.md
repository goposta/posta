---
sidebar_position: 2
title: Platform Settings
description: Configure platform-wide settings
---

# Platform Settings

Administrators can configure platform-wide settings that affect all users via the dashboard or `GET/PUT /api/v1/admin/settings`.

:::tip
For the full request/response schema, see the interactive API Reference at `/docs` on your Posta instance.
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
| `retention_days` | int | `30` | Days to retain email records (metadata). Upper bound for the content windows below |
| `email_body_retention_days` | int | `retention_days` | Days to retain email body content (HTML/text). Capped at `retention_days` |
| `email_attachment_retention_days` | int | `retention_days` | Days to retain attachments and raw inbound messages. Capped at `retention_days` |
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

### Email retention layers

Email data has two very different cost profiles. The **record** — subject, sender,
recipients, status, timestamps — is tiny and worth keeping around for a searchable
history. The **content** — HTML/text bodies, attachments, and (for inbound mail) the raw
`.eml` — is what actually fills the disk. Posta retains them on independent schedules, so
you can keep a long, lightweight log while purging bulk content much sooner.

Three windows apply to every email, all measured from its creation time:

| Layer | Setting | What it removes | What survives |
|-------|---------|-----------------|---------------|
| Record | `retention_days` | the whole row — metadata **and** content | nothing |
| Body | `email_body_retention_days` | HTML + text body | the record (metadata) |
| Attachments | `email_attachment_retention_days` | attachments + their stored bytes | the record (metadata) |

The body and attachment layers **scrub** content in place: the row stays, so the email
still appears in dashboard lists and detail views with its metadata intact — only the
body/attachment fields come back empty. Only the record layer removes the row entirely.

#### Example

With `retention_days = 180`, `email_body_retention_days = 30`, and
`email_attachment_retention_days = 30`:

- **Day 0–30** — full email: metadata, body, and attachments.
- **Day 31–180** — metadata only: body and attachments are gone, but the email still
  shows up in the log with its subject, sender, and status.
- **Day 181+** — the record itself is deleted.

#### Rules

- **Defaults & upgrades.** Both content windows default to the current value of
  `retention_days`, so a fresh install — and any upgrade of an existing one — behaves
  exactly as before (everything purged together) until an admin deliberately shortens a
  content window.
- **Upper bound.** Content windows are **capped at `retention_days`**: a larger value has
  no effect, because the row and all its content are deleted first. The dashboard clamps
  the inputs to `retention_days`, and the cleanup job enforces the same cap server-side.
- **Body and attachments are independent.** Either may be kept longer or shorter than the
  other. The one coupling is the inbound raw `.eml`, which contains *both*: it is purged
  at the **shorter** of the two windows, so it can never preserve content past either
  type's own retention.
- **Not the same as redaction.** [`email_content_visibility`](#available-settings) only
  *hides* body content from the dashboard and API responses — the data is still stored at
  rest. The retention windows above actually *delete* it from the database and blob
  storage. Use visibility for day-to-day privacy in the UI, and the content windows to
  reduce what is stored on disk.

The retention cleanup job runs once daily at 03:00 UTC, so content is purged within
roughly 24 hours of crossing a window.
