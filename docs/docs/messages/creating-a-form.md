---
sidebar_position: 2
title: Creating a form
description: Create and configure a Posta web form endpoint
---

# Creating a Form

Create a form under **Messages → Forms → New form** in the dashboard, or through the API:

```bash
curl -X POST https://posta.example.com/api/v1/workspaces/current/forms \
  -H "Authorization: Bearer $POSTA_API_KEY" \
  -H "X-Posta-Workspace-Id: 1" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Contact form",
    "allowed_origins": ["https://example.com", "https://www.example.com"],
    "notify_emails": ["team@example.com"],
    "reply_from": "support@example.com"
  }'
```

The response includes `public_key` — the unguessable 22-character string that appears in your embed URL.

:::note
Form configuration endpoints require the `admin` API-key scope, not `write`. A form row holds notification recipients and the reply sender, so a content-level key has no business changing it. Reading and replying to messages needs only `read`/`write`.
:::

## Ingest surface

| Field | Default | Notes |
|-------|---------|-------|
| `allowed_origins` | _(empty)_ | Blank accepts any origin. Otherwise an exact `scheme://host[:port]` match. |
| `strict_origin` | `false` | Reject submissions that send no `Origin` header at all (curl, server-to-server). |
| `redirect_url` | _(empty)_ | Where a no-JS form lands after submitting. |
| `max_body_bytes` | `65536` | Clamped to 1 KiB–1 MiB. |
| `max_fields` | `40` | Clamped to 1–200. |
| `allow_attachments` | `false` | When off, uploaded files in a multipart post are discarded and the text fields still store. |

:::caution
The origin allowlist is a spam speed bump, not a security boundary. A plain `<form method="post">` with the default encoding is a CORS *simple request*, so the browser sends it without a preflight regardless of the allowlist. Treat the endpoint as public, because it is.
:::

## Bot controls

| Field | Default | Notes |
|-------|---------|-------|
| `honeypot_field` | `_gotcha` | A non-empty value rejects the submission outright. The generated embed code includes it. |
| `require_nonce` | `false` | Requires a short-lived signed token from `GET /api/v1/f/{key}/nonce`. Single-use, and breaks no-JS forms. |
| `min_fill_seconds` | `3` | Minimum age of the nonce at submit time. Only applies when `require_nonce` is on. |

## Scanning thresholds

| Field | Default | Effect at or above the score |
|-------|---------|------------------------------|
| `flag_threshold` | `3` | Stored and notified, marked for review. |
| `quarantine_threshold` | `6` | Stored, not notified, not dispatched as `message.received`. |
| `reject_threshold` | `10` | Stored for audit only. Hidden from the inbox. |

Thresholds must satisfy `flag ≤ quarantine ≤ reject`. Set `scan_enabled: false` to store everything untouched.

## Notifications

| Field | Default | Notes |
|-------|---------|-------|
| `notify_enabled` | `true` | Master switch for this form. |
| `notify_emails` | _(empty)_ | Blank notifies workspace owners and admins, honouring each user's **Web form messages** preference. At most 10 addresses. |
| `notify_mode` | `immediate` | `immediate`, `hourly`, `daily`, or `off`. Digests are assembled by the `message-digest` cron job. |
| `notify_on_flagged` | `true` | Whether flagged messages also generate a notification. |

A busy contact form on `immediate` can send a lot of mail. Switching to `hourly` is usually what keeps the feature switched on.

## Replying

| Field | Notes |
|-------|-------|
| `reply_from` | Must be an address on an ownership-verified domain in this workspace. Validated when you save, not just when you send. |
| `reply_from_name` | Display name used on outgoing replies. |

Without `reply_from`, the dashboard shows the message but disables the reply composer.

## Rotating the public key

`POST /workspaces/current/forms/{id}/rotate-key` issues a new key. Existing embeds stop working immediately — update your site first, or expect a gap.
