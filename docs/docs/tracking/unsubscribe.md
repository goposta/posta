---
sidebar_position: 2
title: Unsubscribe & One-Click
description: Hosted unsubscribe pages, RFC 8058 one-click unsubscribe, and List-Unsubscribe headers
---

# Unsubscribe & One-Click

Posta hosts the unsubscribe flow so recipients (and their mailbox providers) can opt
out without an account. All of these endpoints live under `/api/v1/t/*` and are
**public — no authentication**. They carry an HMAC-signed token instead of an
`Authorization` header.

:::info Public endpoints
These URLs are opened by recipients and POSTed to by mailbox providers (Gmail,
Apple Mail, Outlook, …) acting on the recipient's behalf. There is no API key or JWT
involved. A missing, malformed, or expired token returns a `404` page.
:::

## Campaign unsubscribe (hosted page + confirm)

```
GET  /api/v1/t/u/{token}
POST /api/v1/t/u/{token}
```

This is the link behind `{{ posta_unsubscribe_url }}` for **campaign** sends.

- `GET` renders a small confirmation page showing the recipient's email and a
  "Confirm Unsubscribe" button.
- `POST` (the form submit) performs the opt-out: it suppresses the subscriber on
  that campaign's list, marks the campaign message as unsubscribed, and records an
  unsubscribe event for analytics. Only that list is affected — other lists the
  recipient belongs to are untouched.

The `{token}` is an HMAC-signed token encoding the campaign message ID.

## Transactional one-click unsubscribe (RFC 8058)

```
GET  /api/v1/t/u/tx/{token}
POST /api/v1/t/u/tx/{token}
```

This is the link behind `{{ posta_unsubscribe_url }}` for **transactional** sends and
the target of the RFC 8058 `List-Unsubscribe-Post` header.

- `GET` renders a confirmation page.
- `POST` is the RFC 8058 **one-click** endpoint: it opts the recipient out with no
  further interaction and no session. It is idempotent and safe for a mailbox
  provider to call automatically.

On a `POST`, every recipient on the email is added to the workspace's suppression
list. If the send referenced a Posta-managed unsubscribe list, the suppression is
**scoped to that list** (`list_unsubscribe` kind); otherwise it is a hard global
suppression. Each opt-out also fires an `email.unsubscribed` webhook carrying the
email UUID, the recipient address, and the list ID.

The `{token}` is an HMAC-signed token with a `tx:` prefix that binds it to the
transactional email ID, so it cannot be replayed against the campaign unsubscribe
handler.

## List-Unsubscribe headers on outgoing mail

Posta sets the standard `List-Unsubscribe` (RFC 2369) and `List-Unsubscribe-Post`
(RFC 8058) headers so mailbox providers show a native "Unsubscribe" control.

For a transactional send, configure this with the `unsubscribe` object on
`POST /api/v1/emails/send`:

```json
{
  "from": "news@example.com",
  "to": ["jane@example.com"],
  "subject": "Weekly digest",
  "html": "<p>...</p>",
  "unsubscribe": {
    "list_id": 7
  }
}
```

The `unsubscribe` object supports two mutually exclusive modes:

- **Posta-managed** — set `list_id` to reference an existing unsubscribe list. Posta
  mints the signed one-click URL (`/api/v1/t/u/tx/{token}`), emits both headers, and a
  click suppresses the recipient on that list only. One-click is implied on this path.
- **Caller-managed** — set `url` to your own endpoint (and optionally `mailto`).
  Posta only emits the header; you own the endpoint. Set `one_click: true` to also
  emit `List-Unsubscribe-Post` — this requires an `https` URL.

| Field | Type | Notes |
|---|---|---|
| `list_id` | integer | Reference a Posta `UnsubscribeList`. Mutually exclusive with `url`. |
| `url` | string | Caller-managed unsubscribe endpoint / RFC 8058 POST target. Mutually exclusive with `list_id`. |
| `mailto` | string | Optional `mailto:` URI emitted alongside `url` (RFC 2369). A bare address is accepted; Posta prepends `mailto:`. |
| `one_click` | boolean | Emit `List-Unsubscribe-Post: List-Unsubscribe=One-Click` (RFC 8058). Applies to the `https` URL target only; implied on the `list_id` path. |

On the wire Posta emits the mailto first, then the https URL:

```
List-Unsubscribe: <mailto:unsubscribe@example.com>, <http://localhost:9000/api/v1/t/u/tx/...>
List-Unsubscribe-Post: List-Unsubscribe=One-Click
```

For **campaign** sends, Posta automatically points the `List-Unsubscribe` header at
the campaign unsubscribe URL (`/api/v1/t/u/{token}`) and enables one-click — there is
nothing to configure per campaign.

:::note Deprecated fields
The top-level `list_unsubscribe_url` and `list_unsubscribe_post` fields are
deprecated. Use the `unsubscribe` object instead.
:::

## Relationship to the unsubscribe URL system variable

The `{{ posta_unsubscribe_url }}` system variable resolves to the right hosted
endpoint depending on the send type: the campaign unsubscribe page for campaigns, and
the RFC 8058 one-click endpoint for transactional sends. The link is only generated
once the message identity is known, so it renders as its own name in template
previews. See [System Variables](../templates/system-variables.md).

## Unsubscribe lists and suppression

Posta-managed unsubscribe lists are workspace-scoped resources you manage via the API:

```
POST   /api/v1/workspaces/current/unsubscribe-lists
GET    /api/v1/workspaces/current/unsubscribe-lists
GET    /api/v1/workspaces/current/unsubscribe-lists/{id}
PUT    /api/v1/workspaces/current/unsubscribe-lists/{id}
DELETE /api/v1/workspaces/current/unsubscribe-lists/{id}
```

A list has a `name`, an optional `public_name` and `description`, and an `active`
flag. Referencing one by `list_id` on a send lets Posta scope a one-click opt-out to
that list.

When a recipient unsubscribes, they land on your **suppression list** so future sends
skip them. See [Suppression List](../contacts/suppression-list.md) for managing
suppressions and [Contact Management](../contacts/contact-management.md) for how
suppression interacts with contacts.
