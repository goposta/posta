---
sidebar_position: 3
title: Configuration
description: Environment variables and configuration options
---

# Configuration

Posta is configured via environment variables. All variables are prefixed with `POSTA_`.

## Server

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTA_PORT` | `9000` | HTTP server port |
| `POSTA_ENV` | `dev` | Environment name |
| `POSTA_DEV_MODE` | `false` | Development mode — stores emails without sending |
| `POSTA_WEB_DIR` | `web/dist` | Path to the dashboard frontend build |
| `POSTA_WEB_URL` | — | Public base URL of the Posta instance |
| `POSTA_API_URL` | — | Public API base URL advertised in the OpenAPI `servers` list (optional if `POSTA_WEB_URL` is set) |

## Database (PostgreSQL)

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTA_DB_HOST` | `localhost` | Database host |
| `POSTA_DB_PORT` | `5432` | Database port |
| `POSTA_DB_USER` | `posta` | Database user |
| `POSTA_DB_PASSWORD` | `posta` | Database password |
| `POSTA_DB_NAME` | `posta` | Database name |
| `POSTA_DB_SSL_MODE` | `disable` | SSL mode (`disable`, `require`, `verify-full`) |
| `POSTA_DB_URL` | — | Full connection string (overrides individual settings) |

## Redis

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTA_REDIS_URL` | — | Full connection string, e.g. `redis://user:pass@host:6379/2`. When set, it overrides `POSTA_REDIS_ADDR`, `POSTA_REDIS_USERNAME`, `POSTA_REDIS_PASSWORD` and `POSTA_REDIS_DB`. |
| `POSTA_REDIS_ADDR` | `localhost:6379` | Redis address (`host:port`) |
| `POSTA_REDIS_USERNAME` | — | Redis ACL username (Redis 6+) |
| `POSTA_REDIS_PASSWORD` | — | Redis password |
| `POSTA_REDIS_DB` | `0` | Redis database number to select |

## Security

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTA_JWT_SECRET` | — | **Required.** JWT signing key. Must be changed in production. |
| `POSTA_ADMIN_EMAIL` | `admin@example.com` | Initial admin account email |
| `POSTA_ADMIN_PASSWORD` | `admin1234` | Initial admin account password |
| `POSTA_CORS_ORIGINS` | `*` | Comma-separated allowed CORS origins |
| `POSTA_ENCRYPTION_KEY` | — | AES-256-GCM key used to encrypt stored SMTP passwords. Falls back to base64 encoding only when empty. |
| `POSTA_EMAIL_VERIFICATION_REQUIRED` | `false` | Require new users to confirm their email address before they can sign in |

## Features

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTA_OPENAPI_DOCS` | `true` | Enable Swagger/ReDoc API documentation |
| `POSTA_METRICS_ENABLED` | `false` | Enable Prometheus metrics endpoint |

:::note
New-user registration is toggled at runtime from **Admin → Settings** (the `registration_enabled` setting), not via an environment variable.
:::

## Rate Limiting

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTA_AUTH_RATE_LIMIT_ENABLED` | `true` | Enable rate limiting on login/register endpoints |
| `POSTA_RATE_LIMIT_HOURLY` | `100` | Maximum emails per hour per user |
| `POSTA_RATE_LIMIT_DAILY` | `1000` | Maximum emails per day per user |

## Worker

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTA_EMBEDDED_WORKER` | `false` | Run the worker within the API server process |
| `POSTA_WORKER_CONCURRENCY` | `10` | Number of worker goroutines |
| `POSTA_WORKER_MAX_RETRIES` | `5` | Maximum retry attempts per email |

## Webhooks

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTA_WEBHOOK_MAX_RETRIES` | `3` | Maximum webhook delivery retries |
| `POSTA_WEBHOOK_TIMEOUT_SECS` | `10` | Webhook HTTP request timeout (seconds) |
| `POSTA_WEBHOOK_PROXY_URL` | — | Optional HTTP/HTTPS/SOCKS5 proxy for outbound webhook delivery |

## Delivery

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTA_AUTO_SUPPRESS_ON_REJECT` | `true` | Add a recipient to the suppression list (and stop retrying) after a permanent `5xx` rejection at `RCPT TO`, e.g. `550 user unknown` |

## Email Verification

Controls the `POST /api/v1/emails/verify` endpoint (syntax, MX, disposable & role-account checks). Results are cached in Redis.

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTA_EMAIL_VERIFY_ENABLED` | `true` | Enable the email verification endpoint |
| `POSTA_EMAIL_VERIFY_CACHE_TTL_HOURS` | `168` | How long an address-level result is cached, in hours (default 7 days) |
| `POSTA_EMAIL_VERIFY_MX_CACHE_TTL_HOURS` | `24` | How long a domain's MX lookup is cached, in hours |
| `POSTA_EMAIL_VERIFY_RATE_HOURLY` | `1000` | Per-user hourly cap on verification requests (`0` disables the limit) |

## OAuth / SSO

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTA_GOOGLE_OAUTH_CLIENT_ID` | — | Google OAuth client ID for SSO login |
| `POSTA_GOOGLE_OAUTH_CLIENT_SECRET` | — | Google OAuth client secret |
| `POSTA_OAUTH_CALLBACK_URL` | — | OAuth callback base URL (optional if `POSTA_WEB_URL` is set) |

## System SMTP

Outbound SMTP server used for platform notifications (daily reports, invitations, alerts). `HOST` and `FROM` must both be set for it to activate.

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTA_SYSTEM_SMTP_HOST` | — | SMTP server host |
| `POSTA_SYSTEM_SMTP_PORT` | `587` | SMTP server port |
| `POSTA_SYSTEM_SMTP_USERNAME` | — | SMTP username |
| `POSTA_SYSTEM_SMTP_PASSWORD` | — | SMTP password |
| `POSTA_SYSTEM_SMTP_FROM` | — | From address for platform notifications |
| `POSTA_SYSTEM_SMTP_ENCRYPTION` | `starttls` | Encryption mode: `none`, `ssl`, or `starttls` |

## Blob Storage

Where email attachments are stored. Leave `POSTA_BLOB_PROVIDER` empty to disable external attachment storage.

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTA_BLOB_PROVIDER` | — | Storage backend: `s3` or `filesystem` |
| `POSTA_BLOB_S3_ENDPOINT` | — | S3-compatible endpoint (e.g. MinIO, R2) |
| `POSTA_BLOB_S3_REGION` | `us-east-1` | S3 region |
| `POSTA_BLOB_S3_BUCKET` | — | S3 bucket name |
| `POSTA_BLOB_S3_ACCESS_KEY` | — | S3 access key |
| `POSTA_BLOB_S3_SECRET_KEY` | — | S3 secret key |
| `POSTA_BLOB_S3_USE_SSL` | `true` | Connect to S3 over TLS |
| `POSTA_BLOB_S3_PATH_STYLE` | `false` | Use path-style addressing (required by some MinIO setups) |
| `POSTA_BLOB_FS_PATH` | `data/attachments` | Storage path when using the `filesystem` provider |

## Inbound Email

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTA_INBOUND_ENABLED` | `false` | Master toggle — enables the SMTP receiver and the `/api/v1/inbound/*` routes |
| `POSTA_INBOUND_SMTP_HOST` | `0.0.0.0` | Bind address for the built-in SMTP receiver |
| `POSTA_INBOUND_SMTP_PORT` | `2525` | SMTP listener port (use `25` publicly) |
| `POSTA_INBOUND_HOSTNAME` | `posta.local` | Hostname announced in EHLO / used as TLS SNI — should match the MX record |
| `POSTA_INBOUND_MAX_MESSAGE_SIZE` | `26214400` | Max raw message size in bytes (default 25 MiB) |
| `POSTA_INBOUND_MAX_ATTACH_SIZE` | `10485760` | Max per-attachment size in bytes (default 10 MiB) |
| `POSTA_INBOUND_WEBHOOK_SECRET` | — | Shared secret for the MX-provider webhook at `POST /api/v1/inbound/webhook` (sent via `X-Posta-Inbound-Secret`) |
| `POSTA_INBOUND_TLS_MODE` | `none` | SMTP TLS mode: `none` or `starttls` |
| `POSTA_INBOUND_TLS_CERT_FILE` | — | PEM cert path (required when TLS mode is `starttls`) |
| `POSTA_INBOUND_TLS_KEY_FILE` | — | PEM key path (required when TLS mode is `starttls`) |
| `POSTA_INBOUND_SMTP_RATE_LIMIT` | `60` | Per-IP max SMTP sessions per window (`0` disables) |
| `POSTA_INBOUND_SMTP_RATE_WINDOW` | `60` | Rate-limit window in seconds |

## Advanced

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTA_ALLOW_DOWNGRADE` | `false` | Allow the server to boot when the binary version is older than the version recorded in the database |
| `POSTA_PLAN_ENFORCEMENT` | `false` | Enforce hosted plan limits / quotas |

## Example `.env` File

```bash
# Server
POSTA_PORT=9000
POSTA_ENV=production

# Database
POSTA_DB_HOST=localhost
POSTA_DB_USER=posta
POSTA_DB_PASSWORD=secure-password
POSTA_DB_NAME=posta
POSTA_DB_PORT=5432

# Redis
POSTA_REDIS_ADDR=localhost:6379

# Security
POSTA_JWT_SECRET=your-very-long-random-secret-key
POSTA_ADMIN_EMAIL=admin@yourdomain.com
POSTA_ADMIN_PASSWORD=strong-admin-password
POSTA_CORS_ORIGINS=https://dashboard.yourdomain.com

# Features
POSTA_METRICS_ENABLED=true

# Rate Limiting
POSTA_AUTH_RATE_LIMIT_ENABLED=true
POSTA_RATE_LIMIT_HOURLY=500
POSTA_RATE_LIMIT_DAILY=5000

# Worker
POSTA_EMBEDDED_WORKER=true
POSTA_WORKER_CONCURRENCY=20
```
