---
sidebar_position: 3
title: System Variables
description: Built-in {{ posta_* }} template variables for unsubscribe and web-view links
---

# System Variables

Posta reserves the `{{ posta_* }}` variable namespace for values it injects into
every templated email. You don't pass these in `template_data` — Posta fills them
in per message, after the message identity is known. Using them is opt-in: a
variable only appears if your template references it.

> The `posta_` prefix is reserved. Any key you supply in `template_data` that starts
> with `posta_` is ignored in favour of the system value.

## Available variables

| Variable | Description |
|---|---|
| `{{ posta_web_view_url }}` | Signed link to read this email on the web ("view in browser"). |
| `{{ posta_mail_web_link }}` | Alias for `posta_web_view_url`. |
| `{{ posta_unsubscribe_url }}` | One-click unsubscribe link for this message. |

## View in browser

```html
<a href="{{ posta_web_view_url }}">View this email in your browser</a>
```

Works in the text part too:

```
View online: {{ posta_web_view_url }}
```

The link is an HMAC-signed, **expiring** capability bound to the message's opaque
ID. The hosted page renders the exact HTML that was sent, on Posta's public web
origin (`POSTA_WEB_URL`), with a restrictive content-security policy, `noindex`, and
no cookies. Opening it does **not** count as an email open.

Notes:
- Links expire after 90 days by default.
- `cid:` inline-attachment images don't resolve on the web — use `https`/`data:` image URLs if you rely on the web view.

## Unsubscribe

```html
<a href="{{ posta_unsubscribe_url }}">Unsubscribe</a>
```

For campaign sends this unsubscribes the recipient from that campaign's list. For
transactional sends it adds the recipient to your suppression list. The link is the
RFC 8058 one-click endpoint, so it also works from the mailbox-provider
"Unsubscribe" button.

## Behaviour in previews

In the template editor / preview there is no real message yet, so each system
variable renders as **its own name** (e.g. `{{ posta_web_view_url }}` →
`posta_web_view_url`) rather than a generated link. This lets a preview of a
template that uses system variables render without errors, and shows the author
which variable sits where. The real links are only generated when the message is
actually sent.
