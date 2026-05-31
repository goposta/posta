---
sidebar_position: 7
title: Email Verification
description: Validate an email address before sending
---

# Email Verification

Check whether an email address is valid and likely deliverable before you send to it. Verification runs a series of cheap-to-expensive checks (syntax, your suppression/bounce history, disposable/role detection, and an MX lookup) and caches results to avoid repeated lookups.

```
POST /api/v1/emails/verify
```

:::tip
For the full request/response schema, see the interactive [API Reference](https://app.goposta.dev/docs).
:::

:::note
No SMTP probe is performed, so mailbox existence is never confirmed — `checks.smtp` is always reported as `"skipped"`. A `valid` result means the address is syntactically correct and the domain accepts mail, not that the specific mailbox exists.
:::

## Request

```bash
curl -X POST http://localhost:9000/api/v1/emails/verify \
  -H "Authorization: Bearer your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com"
  }'
```

| Field | Type | Description |
|-------|------|-------------|
| `email` | string (required) | The address to verify. Validated as an email format; a syntactically malformed address is rejected with `400` before any lookup. |

### Query parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `fresh` | bool | When `true`, bypasses the cache and re-checks the address. Example: `?fresh=true`. |

## Checks performed

Verification runs the following checks in order, returning early once the verdict is conclusive:

1. **Syntax** — the address is parsed; a malformed address is conclusively `invalid`.
2. **Suppression / bounce history** (per workspace, no network) — if the address is on your suppression list or has previously hard-bounced for you, it is conclusively `invalid` for your account.
3. **Disposable detection** — the domain is matched against a known set of throwaway/disposable providers. A match short-circuits to `disposable` without a DNS lookup.
4. **Role-account detection** — the local part is matched against role addresses (e.g. `info`, `admin`, `support`); a match downgrades the verdict to `risky`.
5. **MX lookup** — the domain's mail exchangers are resolved. A domain with no MX (and no A/AAAA fallback) is `invalid`. Per RFC 5321, a domain with only an A/AAAA record is treated as having an implicit MX.

The verdict precedence is: invalid syntax → disposable → no MX → role account (risky) → valid.

## Cache behavior

- The **intrinsic** result (syntax, disposable, role, MX) is cached in Redis per address, and MX answers are cached per domain. The same address and domain are not re-checked on every call.
- The **per-workspace** flags (`suppressed`, `previously_bounced`) are always re-evaluated on each request and layered onto the (possibly cached) intrinsic result, so they reflect your current state even on a cache hit.
- `cached` is `true` in the response when the intrinsic result came from the cache.
- Pass `?fresh=true` to bypass the cache and recompute the intrinsic result.

## Response

```json
{
  "success": true,
  "data": {
    "email": "user@example.com",
    "status": "valid",
    "score": 90,
    "checks": {
      "syntax": true,
      "mx": true,
      "disposable": false,
      "role_account": false,
      "smtp": "skipped"
    },
    "reason": "",
    "mailbox_verified": false,
    "suppressed": false,
    "previously_bounced": false,
    "cached": false,
    "checked_at": "2026-05-31T12:00:00Z"
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `email` | string | The normalized (lowercased, trimmed) address that was checked. |
| `status` | string | Overall verdict: `valid`, `invalid`, `risky`, `disposable`, or `unknown`. |
| `score` | int | Confidence score (0–100). Higher is better; e.g. `90` valid, `60` role account, `10` disposable, `0` invalid. |
| `checks` | object | Individual check outcomes (see below). |
| `reason` | string | Human-readable explanation when the address is not plainly valid. Omitted when empty. |
| `mailbox_verified` | bool | Always `false` — no SMTP RCPT probe is performed. |
| `suppressed` | bool | The address is on your workspace's suppression list. |
| `previously_bounced` | bool | The address has previously hard-bounced for your workspace. |
| `cached` | bool | Whether the intrinsic result was served from cache. |
| `checked_at` | string | Timestamp (UTC) of when the result was computed. |

### `checks` object

| Field | Type | Description |
|-------|------|-------------|
| `syntax` | bool | The address is syntactically valid. |
| `mx` | bool | The domain has resolvable MX (or A/AAAA fallback) records. |
| `disposable` | bool | The domain is a known disposable/throwaway provider. |
| `role_account` | bool | The local part is a role address (e.g. `info@`, `admin@`). |
| `smtp` | string | Always `"skipped"` — no SMTP probe is performed. |

### Status values

| Status | Meaning |
|--------|---------|
| `valid` | Syntax OK and the domain accepts mail (mailbox not probed). |
| `invalid` | Definitely undeliverable (bad syntax, no MX, suppressed, or previously bounced). |
| `risky` | Deliverable but discouraged, e.g. a role-based address. |
| `disposable` | A throwaway/disposable email provider. |
| `unknown` | Could not be determined (e.g. a transient DNS error). |

## Errors

| Status | When |
|--------|------|
| `400` | The `email` field is missing or not a valid email format. |
| `404` | Email verification is disabled on this instance. |
| `429` | The per-user hourly verification rate limit was exceeded. |
