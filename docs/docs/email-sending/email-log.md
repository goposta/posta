---
sidebar_position: 7
title: Email Log
description: List, search, and filter a workspace's sent email
---

# Email Log

Browse the emails a workspace has sent. This workspace-scoped endpoint backs the
**Emails** page in the dashboard and supports pagination plus an operator-based
search string. All routes require a JWT and the workspace header:

```
Authorization: Bearer <jwt>
X-Posta-Workspace-Id: 1
```

:::tip
For the full request/response schema, see the interactive [API Reference](https://app.goposta.dev/docs).
:::

## List Emails

```
GET /api/v1/workspaces/current/emails
```

Returns a paginated list scoped to the current workspace, newest first.

Query parameters:

| Param | Type | Description |
|-------|------|-------------|
| `page` | int | Zero-based page index (default `0`). |
| `size` | int | Page size, 1–100 (default `20`). |
| `q` | string | Search string with optional operators (see below). |
| `sort` | string | Sort key (see [Sorting](#sorting)); default is newest first. |

```bash
curl "http://localhost:9000/api/v1/workspaces/current/emails?page=0&size=20" \
  -H "Authorization: Bearer <jwt>" \
  -H "X-Posta-Workspace-Id: 1"
```

## Search operators

The `q` parameter is a single search string that accepts operators. Words
without an operator match the **subject** — the body is not searched (it may be
stored as a blob or redacted).

| Operator | Matches | Example |
|----------|---------|---------|
| _(bare text)_ | Subject | `invoice reminder` |
| `subject:` | Subject | `subject:"weekly report"` |
| `from:` | Sender address | `from:alice@acme.com` |
| `to:` | Any recipient | `to:bob@example.com` |
| `template:` | Template name | `template:welcome` |
| `has:attachment` | Emails that have attachments | `has:attachment` |
| `status:` | One or more statuses (comma-separated) | `status:failed,suppressed` |
| `after:` | Created on/after a date | `after:2026-07-01` |
| `before:` | Created on/before a date (inclusive) | `before:2026-07-20` |

**Matching rules**

- Text operators (`from:`, `to:`, `subject:`, `template:`) and bare words use
  **case-insensitive substring** matching.
- Use `*` as a wildcard: `to:*@example.com` matches addresses ending in
  `@example.com`; `subject:invoice*` matches subjects that start with "invoice".
- Wrap multi-word values in quotes: `subject:"payment failed"`.
- `status:` accepts a comma-separated list; an email matches if it is in **any**
  of the listed statuses. See [Email Status](/docs/email-sending/email-status)
  for the status values.
- Dates bound `created_at`. A `YYYY-MM-DD` value is treated as a whole day, so
  `before:` is inclusive of that day; a full RFC3339 timestamp is used as the
  exact instant.
- Operators combine with **AND**. Any bare words are merged into the subject search.

## Sorting

The `sort` parameter orders the results. Prefix a field with `-` for descending.
Any unknown value falls back to the default (`-created_at`, newest first), so the
default ordering is always safe.

| Sort key | Orders by |
|----------|-----------|
| `created_at` / `-created_at` | Creation time (default: `-created_at`) |
| `sent_at` / `-sent_at` | Delivery time |
| `subject` / `-subject` | Subject |
| `sender` / `-sender` | Sender address |
| `template` / `-template` | Template name |
| `status` / `-status` | Status |

```bash
curl "http://localhost:9000/api/v1/workspaces/current/emails?sort=-sent_at" \
  -H "Authorization: Bearer <jwt>" \
  -H "X-Posta-Workspace-Id: 1"
```

Example — failed or suppressed invoices that carry an attachment, from July 2026:

```bash
curl -G http://localhost:9000/api/v1/workspaces/current/emails \
  -H "Authorization: Bearer <jwt>" \
  -H "X-Posta-Workspace-Id: 1" \
  --data-urlencode "q=invoice has:attachment status:failed,suppressed after:2026-07-01 before:2026-07-31"
```

## In the dashboard

The **Emails** page exposes the same capability: a search box that accepts every
operator above, plus quick filters for status (multi-select), a date range, and
an attachments toggle. Clicking a column header sorts by it (click again to
reverse, once more to clear). The active query and sort are stored in the page
URL, so a view can be bookmarked or shared, and it survives reloads and
pagination. Times are shown in your browser's local timezone with a short label
(e.g. `GMT+3`).

## Email details

Fetch the full record for one email by its UUID:

```
GET /api/v1/workspaces/current/emails/{id}
```

Email content may be redacted depending on the administrator's privacy settings.
To retry a failed send, see [Email Status](/docs/email-sending/email-status).
