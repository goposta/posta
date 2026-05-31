---
sidebar_position: 2
title: Sending & Lifecycle
description: Send, pause, resume, and cancel campaigns
---

# Sending & Lifecycle

Manage the full lifecycle of a campaign: send, pause, resume, cancel, and duplicate.

:::tip
For the full request/response schema, see the interactive [API Reference](https://app.goposta.dev/docs).
:::

## Sending a Campaign

```
POST /api/v1/workspaces/current/campaigns/{id}/send
```

No request body required. The campaign transitions from `draft` to:

- **`sending`** — if no `scheduled_at` is set or the scheduled time is in the past
- **`scheduled`** — if `scheduled_at` is in the future

Posta processes the subscriber list and queues individual emails for delivery. Each subscriber in the list receives one email.

```bash
curl -X POST http://localhost:9000/api/v1/workspaces/current/campaigns/42/send \
  -H "Authorization: Bearer <jwt>" \
  -H "X-Posta-Workspace-Id: 1"
```

### Send Rate Control

If `send_rate` is set on the campaign, Posta throttles delivery to the specified number of emails per minute. This helps avoid overwhelming SMTP servers or hitting provider rate limits.

### Local Time Sending

When `send_at_local_time` is `true` and `scheduled_at` is set, Posta delivers the campaign at the scheduled time in each subscriber's local timezone. Subscribers without a timezone receive the email at the scheduled UTC time.

## Pausing a Campaign

```
POST /api/v1/workspaces/current/campaigns/{id}/pause
```

Transitions a `sending` campaign to `paused`. Emails already queued will still be delivered, but no new emails are queued.

```bash
curl -X POST http://localhost:9000/api/v1/workspaces/current/campaigns/42/pause \
  -H "Authorization: Bearer <jwt>" \
  -H "X-Posta-Workspace-Id: 1"
```

## Resuming a Campaign

```
POST /api/v1/workspaces/current/campaigns/{id}/resume
```

Transitions a `paused` campaign back to `sending`. Remaining subscribers will be processed.

```bash
curl -X POST http://localhost:9000/api/v1/workspaces/current/campaigns/42/resume \
  -H "Authorization: Bearer <jwt>" \
  -H "X-Posta-Workspace-Id: 1"
```

## Cancelling a Campaign

```
POST /api/v1/workspaces/current/campaigns/{id}/cancel
```

Permanently cancels a campaign. Works from `sending`, `paused`, or `scheduled` states. Cancelled campaigns cannot be resumed.

```bash
curl -X POST http://localhost:9000/api/v1/workspaces/current/campaigns/42/cancel \
  -H "Authorization: Bearer <jwt>" \
  -H "X-Posta-Workspace-Id: 1"
```

## Duplicating a Campaign

```
POST /api/v1/workspaces/current/campaigns/{id}/duplicate
```

Creates a copy of the campaign with `(copy)` appended to the name. The new campaign starts in `draft` status with all settings preserved.

```bash
curl -X POST http://localhost:9000/api/v1/workspaces/current/campaigns/42/duplicate \
  -H "Authorization: Bearer <jwt>" \
  -H "X-Posta-Workspace-Id: 1"
```

## Status Transitions

```
draft → sending (immediate send)
draft → scheduled (future send)
scheduled → sending (when scheduled time arrives)
sending → paused
paused → sending (resume)
sending → cancelled
paused → cancelled
scheduled → cancelled
sending → sent (all messages processed)
```
