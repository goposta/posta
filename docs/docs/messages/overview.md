---
sidebar_position: 1
title: Overview
description: Receive website form submissions in Posta and reply to them from the dashboard
---

# Web Form Messages

Messages let a website contact form post directly to Posta. Each submission is scanned, stored, and pushed to your team — and you reply to the sender from the dashboard, through the same sending pipeline, domains, and suppression list as the rest of your email.

There is no auto-responder. Every reply is written by a person.

## Enabling messages

Messages are off by default.

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTA_MESSAGES_ENABLED` | `false` | Master switch for the feature and the public ingest endpoint. |
| `POSTA_MESSAGES_MAX_BODY_BYTES` | `65536` | Maximum submission body size. Each form can lower it. |
| `POSTA_MESSAGES_MAX_ATTACH_SIZE` | `5242880` (5 MiB) | Maximum size of a single uploaded file. Attachments are off per form by default. |
| `POSTA_MESSAGES_IP_RATE_LIMIT` | `20` | Submissions allowed per IP per window. `0` disables the limiter. |
| `POSTA_MESSAGES_IP_RATE_WINDOW` | `3600` | Rate-limit window in seconds. |
| `POSTA_MESSAGES_PER_FORM_HOURLY` | `200` | Submissions allowed per form per hour. |
| `POSTA_MESSAGES_PER_EMAIL_HOURLY` | `5` | Submissions allowed per sender address per hour, per form. |
| `POSTA_MESSAGES_PER_WORKSPACE_DAILY` | `1000` | Submissions allowed per workspace per day. |
| `POSTA_MESSAGES_INBOUND_DOMAIN` | _(empty)_ | Domain used for `msg+<token>@` reply addressing. Requires `POSTA_INBOUND_ENABLED=true`. |

:::warning
`POSTA_MESSAGES_IP_RATE_LIMIT=0` leaves a public, unauthenticated endpoint with no per-IP ceiling. Posta logs a security warning at startup when you do this.
:::

## Concepts

| Term | Meaning |
|------|---------|
| **Form** | The endpoint definition: public key, allowed origins, spam posture, notification recipients, reply sender. |
| **Message** | One submission, with the fields as submitted and the scan verdict. |
| **Reply** | One operator answer, linked to the `Email` record that carried it. |
| **Verdict** | The scanner's output: a score, the reasons behind it, and an action. |

A workspace can have as many forms as it needs — contact, support, careers — each with its own configuration.

## How it works

```
website form  ──►  POST /api/v1/f/{public_key}  ──►  scan  ──►  stored (Message)
                                                                     │
                                                                     ├─►  notification email  ──►  your team
                                                                     ├─►  message.received webhook  ──►  your endpoint
                                                                     └─►  GET /message-stream (SSE)  ──►  dashboard
```

1. A visitor submits your form. The browser posts URL-encoded, multipart, or JSON data to the form's public endpoint.
2. Posta checks the per-IP limiter, the form's origin allowlist, the honeypot, the optional nonce, and the four Redis rate limiters.
3. The scanner produces a score and an action — `allow`, `flag`, `quarantine`, or `reject`.
4. The message is stored. Rejected messages are never notified, never dispatched, and hidden from the inbox.
5. Notification and webhook dispatch happen asynchronously so a slow mailbox never delays the visitor's response.
6. An operator opens the message in the dashboard and writes a reply. It is sent through the normal email pipeline, so delivery status, opens, and bounces show up in the email log.

## Response contract

The endpoint always answers `202 Accepted` for a submission it stored **or silently rejected**. A spam client gets no signal it can tune against. Only rate limiting (`429`), a disallowed origin (`403`), and malformed payloads (`400`/`413`) produce a different status.

For a JSON request the body is:

```json
{ "success": true, "data": { "id": "b0b1…", "status": "received", "message": "Thanks — your message has been received." } }
```

For a plain HTML form post Posta redirects to `_redirect` (when it is on an allowed host) or the form's configured redirect URL, and otherwise renders a small confirmation page.

## Next steps

- [Creating a form](./creating-a-form.md)
- [Embedding a form](./embedding.md)
- [Spam filtering](./spam-filtering.md)
- [Replying to messages](./replying.md)
