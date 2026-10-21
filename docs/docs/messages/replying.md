---
sidebar_position: 5
title: Replying to messages
description: Answer a form submission from the Posta dashboard
---

# Replying to Messages

Replies are written by a person. Posta has no auto-responder for web forms, deliberately: a public endpoint that emails an arbitrary attacker-supplied address, from your verified domain, is a spam amplifier and a fast route to a blocklisted sending domain.

## Before you can reply

The form needs a `reply_from` address on an ownership-verified domain in the workspace, and the message needs a usable sender address. The dashboard disables the composer and explains which of the two is missing.

Replying requires the **editor** workspace role or higher. A viewer can read the inbox but cannot speak for the workspace.

## From the dashboard

Open a message and write your reply. Posta sends it through the normal email pipeline, which means:

- the workspace suppression list applies
- plan rate limits apply
- delivery status, opens, clicks, and bounces appear in the email log, linked to the reply

The reply is recorded on the thread with its `email_uuid`, so there is no second delivery state machine to keep in sync.

## From the API

```bash
curl -X POST https://posta.example.com/api/v1/workspaces/current/messages/$MESSAGE_UUID/reply \
  -H "Authorization: Bearer $POSTA_API_KEY" \
  -H "X-Posta-Workspace-Id: 1" \
  -H "Content-Type: application/json" \
  -d '{
    "subject": "Re: Question about pricing",
    "text": "Hi Ada — thanks for reaching out...",
    "html": "<p>Hi Ada — thanks for reaching out...</p>"
  }'
```

Omit `subject` and Posta derives `Re: <original subject>`. At least one of `html` or `text` is required.

## Threading

When `POSTA_MESSAGES_INBOUND_DOMAIN` is set (and inbound email is enabled), replies carry:

```
Reply-To: msg+<thread_token>@<inbound_domain>
```

so the sender's email answer routes back onto the same thread instead of into someone's personal mailbox. Without it, replies are one-directional: the visitor's answer goes wherever the `reply_from` mailbox points.

The inbound domain must be a verified domain of the same workspace, because the answer arrives through the normal inbound pipeline. When it does:

- it is attached to the thread as a reply marked **Replied by email**, the message moves back to `open` and becomes unread, and the inbound email is marked `forwarded` instead of firing `email.inbound`;
- the assignee is notified, or the form's usual recipients when nobody is assigned;
- an answer from an address other than the original sender is still attached, but flagged **Different sender**: it may be a forward, or someone using a leaked thread address;
- a thread address only routes within its own workspace.

Every reply gets its own `Message-ID` on the sending domain. The first one becomes the thread root, and later replies set `In-Reply-To` to the latest message on the thread and `References` to the root plus the most recent replies, so mail clients group the whole conversation, both directions included.

These headers are set by Posta itself, so they apply whether or not custom headers are enabled on the platform.

## Message state

Separate from the spam status, each message carries a workflow state:

| State | Meaning |
|-------|---------|
| `new` | Not yet opened. |
| `open` | Opened, not yet answered. Set automatically on first read. |
| `replied` | An operator reply has been sent. |
| `closed` | Done. |
| `spam` | Marked as spam. |

Set it with `PUT /workspaces/current/messages/{id}/state`, and assign a message to a teammate with `PUT /workspaces/current/messages/{id}/assign`.

## Events

| Event | Fires when |
|-------|------------|
| `message.received` | A submission passes scanning (`received` or `flagged`). |
| `message.spam` | A submission is quarantined or rejected. |
| `message.replied` | An operator reply is sent from the dashboard or the API. |
| `message.reply_received` | The sender answers a thread by email (see [Threading](#threading)). |

Subscribe under **Developers → Webhooks**. The payload carries the full submission, the scan verdict, and the reasons behind it — see [event types](../webhooks/event-types.md).

The dashboard also streams `message.*` events over `GET /workspaces/current/message-stream` (SSE), which is what makes the inbox and an open thread update live. Events go through Redis, so they reach the dashboard whichever server replica it is connected to, including events raised by a separate worker process.

## Retention

| Setting | Default | Applies to |
|---------|---------|------------|
| `message_retention_days` | `365` | All messages, plus their attachment blobs. |
| `message_spam_retention_days` | `30` | Quarantined and rejected messages. |

Both are platform settings swept by the `retention` cron job. A form's own `retention_days` can shorten the window for that form; it cannot extend it past `message_retention_days`.

To erase one person's messages on request, use **Workspace settings → Data Management (GDPR) → Delete Message Data**, or `POST /workspaces/current/gdpr/delete-messages` with `{"email": "..."}`. It removes every message from that address, its thread replies, the inbound emails those replies arrived as, and all attachments. API keys need the `admin` scope for every `/gdpr/*` route.
