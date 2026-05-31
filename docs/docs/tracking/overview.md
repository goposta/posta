---
sidebar_position: 1
title: Tracking Overview
description: How Posta tracks email opens and clicks, the hosted web view, and where engagement data lands
---

# Tracking & Engagement

Posta measures engagement by injecting a 1×1 open-tracking pixel and rewriting links
for click tracking when a campaign is sent. It also serves a hosted "view in
browser" page. All of these are served from public, unauthenticated endpoints under
`/api/v1/t/*` so that recipients and mailbox providers can reach them.

:::info Public endpoints
Everything under `/api/v1/t/*` is **public — no authentication**. These URLs are meant
to be opened by email recipients and fetched by mailbox providers. They carry their
own HMAC-signed tokens/signatures instead of an `Authorization` header, so a missing
or tampered signature returns `404`.
:::

## Open tracking

```
GET /api/v1/t/o/{message_id}.gif
```

When a campaign message is built, Posta injects a hidden tracking pixel just before
`</body>`:

```html
<img src="http://localhost:9000/api/v1/t/o/123.gif?sig=..." width="1" height="1" alt="" style="display:none" />
```

Opening the email loads the GIF, and Posta records an **open** event. Details:

- The `sig` query parameter is a mandatory HMAC signature over the message ID.
  A request with no signature or a bad signature returns `404` and records nothing,
  so a third party hitting the predictable `/t/o/{message_id}.gif` path cannot
  inflate your metrics.
- The endpoint always returns a 1×1 transparent GIF with `Cache-Control: no-cache`.
- Requests from known bot user-agents (such as security scanners and link
  pre-fetchers) are served the pixel but **not** counted.
- The first open stamps `opened_at` on the campaign message; every open is also
  stored as an event for total/repeat-open metrics.

## Click tracking

```
GET /api/v1/t/c/{message_id}/{hash}
```

Every `http(s)` link in the HTML body is rewritten to a Posta redirect URL before
the message is sent. A rewritten link looks like:

```
http://localhost:9000/api/v1/t/c/123/ab12cd34ef56gh78?sig=...
```

When the recipient clicks it, Posta records a **click** event and then issues a
`302` redirect to the original destination. Details:

- `{hash}` is a deterministic hash of the campaign + original URL; Posta stores the
  original URL and looks it up on each click.
- The `sig` query parameter is a mandatory HMAC signature; a missing or bad
  signature returns `404`.
- Only `http://` and `https://` destinations are rewritten and redirected. `mailto:`
  and `tel:` links, and any link that already points at Posta's own `/t/` tracking
  paths, are left untouched. Non-http redirect targets are rejected with `400` to
  prevent open-redirect/SSRF abuse.
- Bot user-agents are redirected but not counted.
- The first click stamps `clicked_at` on the message; per-link click counts are
  incremented, and a unique click event is recorded per link per message.

## Enabling and disabling tracking

Tracking is a **server-side capability**, not a per-send or per-workspace toggle:

- For **campaign** sends, open-pixel injection and link rewriting are applied
  automatically to every message whenever the tracking service is configured (it
  requires a public base URL and an HMAC signing key on the server). There is no
  per-campaign or workspace checkbox to turn it off — if tracking is configured, a
  campaign send is tracked.
- For **transactional** sends (`POST /api/v1/emails/send` and friends), Posta does
  **not** rewrite links or inject an open pixel. Transactional messages only gain
  Posta-hosted links when the template references a `{{ posta_* }}` system variable
  (see [System Variables](../templates/system-variables.md)) or when the send names a
  Posta-managed unsubscribe list (see [Unsubscribe & One-Click](./unsubscribe.md)).

## Where the data goes

Open and click events feed **campaign analytics**. The authenticated, workspace-scoped
endpoint returns aggregate counts, per-variant breakdowns (for A/B tests), per-link
click totals, and open/click time series:

```
GET /api/v1/workspaces/current/campaigns/{id}/analytics
```

```bash
curl http://localhost:9000/api/v1/workspaces/current/campaigns/42/analytics \
  -H "Authorization: Bearer <jwt>" \
  -H "X-Posta-Workspace-Id: 1"
```

The response includes `analytics`, optional `variant_analytics`, `links`,
`open_series`, and `click_series`. See [Campaigns](../campaigns/overview.md) for how
these surface in the dashboard.

## View in browser (web view)

```
GET /api/v1/t/v/{token}
```

Posta hosts a "view this email in a browser" page for any sent message. The link is
produced by the `{{ posta_web_view_url }}` system variable (and its
`{{ posta_mail_web_link }}` alias). The `{token}` is an HMAC-signed, **expiring**
capability bound to the email's opaque UUID — it defaults to a 90-day lifetime, and an
invalid or expired token renders a "link is invalid or has expired" page.

The hosted page:

- Renders the exact HTML that was sent (falling back to the text body when there is
  no HTML).
- **Strips the open-tracking pixel**, so loading the web view does not inflate open
  metrics.
- Is served with a restrictive `Content-Security-Policy`, `X-Robots-Tag: noindex,
  nofollow`, `Referrer-Policy: no-referrer`, and no cookies.

See [System Variables](../templates/system-variables.md) for how to add the web-view
link to a template.
