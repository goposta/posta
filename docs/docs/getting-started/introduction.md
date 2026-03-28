---
sidebar_position: 1
title: Introduction
description: What is Posta and why use it
---

# Introduction

**Posta** is a self-hosted, open-source email delivery platform that gives developers full control over their email infrastructure. It provides a developer-friendly REST API for sending emails, managing templates, tracking delivery, and monitoring analytics — all without relying on third-party services like SendGrid or Mailgun.

## Why Posta?

- **Self-hosted** — Your data stays on your servers. No vendor lock-in.
- **Developer-first** — Clean REST API with official SDKs for Go, PHP, and Java.
- **Full-featured** — Templates with versioning and localization, SMTP management, domain verification, webhooks, analytics, and more.
- **Open source** — Licensed under Apache 2.0. Contribute, fork, or customize as needed.

## Key Features

| Feature | Description |
|---------|-------------|
| **Email Delivery** | Send single, template, and batch emails via REST API |
| **Templates** | Version-controlled templates with multi-language support |
| **SMTP Management** | Configure multiple SMTP servers with automatic failover |
| **Domain Verification** | SPF, DKIM, and DMARC record verification |
| **Webhooks** | Real-time notifications on email events |
| **Analytics** | Delivery rates, bounce rates, and latency metrics |
| **Security** | API keys, JWT auth, 2FA (TOTP), rate limiting, IP allowlists |
| **Admin Panel** | User management, platform metrics, shared SMTP pool |
| **GDPR Compliance** | Data export, import, and deletion |
| **Prometheus Metrics** | Built-in observability for production monitoring |

## Architecture

Posta is built with:

- **Go** backend using the Okapi web framework
- **PostgreSQL** for persistent storage
- **Redis** for job queues (via Asynq) and caching
- **Vue 3** dashboard (embedded or standalone)

```
┌─────────────┐     ┌─────────────┐     ┌──────────────┐
│  Your App   │────▶│  Posta API  │────▶│  PostgreSQL  │
│  (SDK/HTTP) │     │  (Go)       │     └──────────────┘
└─────────────┘     │             │     ┌──────────────┐
                    │             │────▶│  Redis       │
                    └──────┬──────┘     └──────────────┘
                           │
                    ┌──────▼──────┐     ┌──────────────┐
                    │   Worker    │────▶│  SMTP Server │
                    │  (Asynq)   │     └──────────────┘
                    └─────────────┘
```

## Next Steps

- [Installation](/docs/getting-started/installation) — Deploy Posta with Docker or from source
- [Configuration](/docs/getting-started/configuration) — Configure environment variables
- [Quick Start](/docs/getting-started/quickstart) — Send your first email in minutes
