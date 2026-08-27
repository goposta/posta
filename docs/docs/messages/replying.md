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

Replies also set `In-Reply-To` and `References` from the thread's root message id, so mail clients group the conversation.

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

Subscribe under **Developers → Webhooks**. The payload carries the full submission, the scan verdict, and the reasons behind it — see [event types](../webhooks/event-types.md).

The dashboard also streams `message.*` events over `GET /workspaces/current/message-stream` (SSE), which is what makes the inbox update live.

## Retention

| Setting | Default | Applies to |
|---------|---------|------------|
| `message_retention_days` | `365` | All messages, plus their attachment blobs. |
| `message_spam_retention_days` | `30` | Quarantined and rejected messages. |

Both are platform settings swept by the `retention` cron job. A form's own `retention_days` can shorten the window for that form.
