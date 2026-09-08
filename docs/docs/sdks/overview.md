---
sidebar_position: 1
title: Overview
description: Official Posta SDKs
---

# Official SDKs

Posta provides official client libraries that cover the whole API, grouped by
resource:

| Area | Covers |
|------|--------|
| Emails | send, templated and batch sends, preview, address verification, status, retry, list |
| Templates | templates, versions, localizations, preview, send-test, import and export |
| Campaigns | CRUD, send, pause, resume, cancel, duplicate, per-recipient messages, analytics |
| Subscribers and lists | CRUD, members, segments, opt-ins and opt-outs, bulk import |
| Suppressions and bounces | list, add, remove, record |
| Sending infrastructure | domains and DNS verification, SMTP servers, SMTP relay credentials |
| Webhooks | endpoints, delivery logs, and signature verification |
| Web forms | forms, embed snippets, submissions, replies, spam filters |
| Inbound email | list, get, retry, raw `.eml`, attachments |
| Analytics | volume, delivery and bounce trends, provider breakdown, dashboard stats |
| Workspaces | members, invitations, settings, SSO, audit log, data export and import, GDPR erasure |
| Account and platform admin | profile, 2FA, sessions, notifications; users, plans, shared servers, settings |

## Available SDKs

| Language | Package | Min Version |
|----------|---------|-------------|
| [Go](/docs/sdks/go) | `github.com/goposta/posta-go` | Go 1.25+ |
| [Node.js](https://github.com/goposta/posta-node) | `@goposta/posta` | Node 18+ |
| [Python](https://github.com/goposta/posta-python) | `posta` | Python 3.8+ |
| [PHP](/docs/sdks/php) | `goposta/posta-php` | PHP 8.1+ |
| [Java](/docs/sdks/java) | `com.github.goposta:posta-java` | Java 11+ |
| [.NET](/docs/sdks/dotnet) | `Posta` | .NET 10+ |

## Credentials

Most machine-facing endpoints authenticate with an API key. Account-level
endpoints (`/users/me/*`) and the platform admin surface accept only a user
session token — an API key is never a valid credential there, whatever scopes
it carries.

Workspace-scoped endpoints resolve the active workspace from the
`X-Posta-Workspace-Id` header. A workspace-bound API key carries its workspace
already; an account-wide key or a user session must name one.

## API key scopes

A key reaches only what its scopes allow:

| Scope | Grants |
|-------|--------|
| `send` | sending, verification, and subscriber-list opt-ins |
| `read` | reading emails, bounces, webhook deliveries, and workspace resources |
| `write` | mutating workspace resources |
| `webhooks` | managing webhook endpoints |
| `admin` | tenant administration: API keys, members, invitations, settings, SSO |
| `*` | every scope |

Note that `send` grants none of the others: a send-only key is confined to the
public send API. A 403 from an endpoint you expect to work usually means a
missing scope.

## Error Handling

All SDKs provide typed API errors that include:

- **HTTP status code** — The response status (e.g., 400, 401, 429)
- **Error info** — Structured error details with `code` and `message` fields

## Response Envelope

All API responses use a standard envelope:

```json
{
  "success": true,
  "data": { ... }
}
```

Error responses:

```json
{
  "success": false,
  "error": {
    "code": "validation_error",
    "message": "Invalid email address",
    "error": "to[0]: invalid format"
  }
}
```
