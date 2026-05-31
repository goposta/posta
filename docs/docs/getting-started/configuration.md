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
| `POSTA_REDIS_ADDR` | `localhost:6379` | Redis address |
| `POSTA_REDIS_PASSWORD` | — | Redis password |

## Security

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTA_JWT_SECRET` | — | **Required.** JWT signing key. Must be changed in production. |
| `POSTA_ADMIN_EMAIL` | `admin@example.com` | Initial admin account email |
| `POSTA_ADMIN_PASSWORD` | `admin1234` | Initial admin account password |
| `POSTA_CORS_ORIGINS` | `*` | Comma-separated allowed CORS origins |

## Features

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTA_REGISTRATION_ENABLED` | `true` | Allow new user registration |
| `POSTA_OPENAPI_DOCS` | `true` | Enable Swagger/ReDoc API documentation |
| `POSTA_METRICS_ENABLED` | `false` | Enable Prometheus metrics endpoint |

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
POSTA_REGISTRATION_ENABLED=false
POSTA_METRICS_ENABLED=true

# Rate Limiting
POSTA_AUTH_RATE_LIMIT_ENABLED=true
POSTA_RATE_LIMIT_HOURLY=500
POSTA_RATE_LIMIT_DAILY=5000

# Worker
POSTA_EMBEDDED_WORKER=true
POSTA_WORKER_CONCURRENCY=20
```
