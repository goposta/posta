---
sidebar_position: 5
title: Rate Limiting
description: Email and API rate limits
---

# Rate Limiting

Posta enforces rate limits to prevent abuse and ensure fair usage.

## Email Rate Limits

Rate limits are applied per user:

| Limit | Default | Environment Variable |
|-------|---------|---------------------|
| Hourly | 100 emails | `POSTA_RATE_LIMIT_HOURLY` |
| Daily | 1,000 emails | `POSTA_RATE_LIMIT_DAILY` |

When a rate limit is exceeded, the API returns `429 Too Many Requests`:

```json
{
  "success": false,
  "error": {
    "code": "rate_limit_exceeded",
    "message": "Hourly email limit exceeded"
  }
}
```

## Login Rate Limiting

Login attempts are rate-limited per IP address to prevent brute force attacks. This is enabled by default and can be toggled via the `POSTA_AUTH_RATE_LIMIT_ENABLED` environment variable.

The client IP is read from `X-Forwarded-For` / `X-Real-IP`. When Posta runs behind a reverse proxy, set `POSTA_TRUSTED_PROXIES` to the proxy's address or CIDR so those headers are only accepted from it; otherwise any client can pick its own IP and sidestep the limit.

## Batch Emails

Each recipient in a batch send counts toward the rate limit. A batch of 100 recipients counts as 100 emails.

## Configuring Limits

Adjust limits via environment variables:

```bash
POSTA_AUTH_RATE_LIMIT_ENABLED=true   # Set to false to disable login/register rate limiting
POSTA_RATE_LIMIT_HOURLY=500
POSTA_RATE_LIMIT_DAILY=5000
```

Administrators can also adjust rate limits dynamically via platform settings.
