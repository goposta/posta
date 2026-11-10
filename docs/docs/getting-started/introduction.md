---
sidebar_position: 1
title: Introduction
description: What is Posta and why use it
---

# Introduction

**Posta** is a self-hosted, open-source email delivery platform that gives developers full control over their email infrastructure. It provides a developer-friendly REST API for sending emails, managing templates, tracking delivery, and monitoring analytics — all without relying on third-party services like SendGrid or Mailgun.

## Why Posta?

- **Self-hosted** — Your data stays on your servers. No vendor lock-in.
- **Developer-first** — Clean REST API with official SDKs for Go, PHP, Java, and .NET.
- **Full-featured** — Templates with versioning and localization, SMTP management, domain verification, webhooks, analytics, and more.
- **Open source** — Licensed under AGPL-3.0. Contribute, fork, or customize as needed.

## Key Features

| Feature | Description |
|---------|-------------|
| **Email Delivery** | Send single, template, and batch emails via REST API with scheduled sending and automatic retries |
| **Email Relay** | Point an existing SMTP client at Posta and relay its mail through the same outbound pipeline as the API |
| **Templates** | Version-controlled templates with multi-language support, variable substitution, and stylesheet inlining |
| **SMTP Management** | Configure multiple SMTP servers with TLS support and shared pools |
| **Domain Verification** | SPF, DKIM, and DMARC record verification with verified sender enforcement |
| **Contacts & Suppression** | Contact tracking, segmentation, bounce handling, and automatic suppression lists |
| **Subscribers & Lists** | Subscriber management with static and dynamic lists, bulk import, and custom fields |
| **Web Forms** | Contact forms with a shared inbox, spam scoring and replies by email, from a widget, a share link or plain HTML |
| **Surveys & Signups** | Surveys, registrations and signups kept as responses, summarised per question, with CSV and JSON exports |
| **Live Chat** | A chat widget for your website with business hours, visitor identity and replies by email once visitors leave |
| **Campaigns** | Template-based bulk sending with scheduling, A/B testing, throttling, and analytics |
| **Webhooks & Events** | Real-time HTTP notifications with retry strategies and delivery tracking |
| **Analytics** | Email delivery metrics, trends, bounce rates, and daily reports |
| **Security** | API keys, JWT auth, 2FA (TOTP), OAuth / SSO, rate limiting, IP allowlists |
| **Workspaces** | Multi-tenant architecture with role-based access and scoped API keys |
| **Admin Panel** | User management, platform metrics, shared SMTP pool, OAuth providers, and platform settings |
| **Dashboard** | Vue-based web UI for managing all resources with dark/light mode |
| **GDPR Compliance** | Data export, import, and deletion |
| **Prometheus Metrics** | Built-in observability for production monitoring |

## API Reference

Every Posta instance serves its own interactive API documentation:

- **API Reference** — `/docs` on your Posta instance
- **OpenAPI spec** — `/openapi.json` on your Posta instance

Running locally with the default port, that is [http://localhost:9000/docs](http://localhost:9000/docs).

## Architecture

Posta ships as a single binary that runs in two modes: `posta` serves the HTTP
API, dashboard, website widgets, and SMTP listeners, while `posta worker` consumes
the job queue. Both are stateless, with all state in PostgreSQL, Redis, and optional
object storage, so servers and workers scale out horizontally.

See [Architecture](/docs/getting-started/architecture) for the full picture —
what each process does, how forms and live chat reach it, how the queues are split,
and how to deploy them.

## Screenshots

See [Screenshots](/docs/getting-started/screenshots) for a tour of the dashboard,
the forms and the live chat widget.

## Next Steps

- [Managed Posta](/docs/getting-started/managed-posta) — deploy your own instance from the Miabi Marketplace, no Docker Compose required
- [Installation](/docs/getting-started/installation) — Deploy Posta with Docker or from source
- [Configuration](/docs/getting-started/configuration) — Configure environment variables
- [Quick Start](/docs/getting-started/quickstart) — Send your first email in minutes
- [Architecture](/docs/getting-started/architecture) — How the pieces fit together and how to deploy them
- [Screenshots](/docs/getting-started/screenshots) — A tour of the dashboard and the website widgets

## Community

Have a question, an idea, or something you've built with Posta? Join the conversation on the [Posta community forum](https://goposta.discourse.group/).
Bugs and feature requests go to [GitHub issues](https://github.com/goposta/posta/issues).
