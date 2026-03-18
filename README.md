# Posta

<p align="center">
  <img src="https://raw.githubusercontent.com/jkaninda/posta/main/logo.png" alt="Posta" width="150" />
</p>

[![CI](https://github.com/jkaninda/posta/actions/workflows/ci.yml/badge.svg)](https://github.com/jkaninda/posta/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/jkaninda/posta)](https://goreportcard.com/report/github.com/jkaninda/posta)
[![Go](https://img.shields.io/github/go-mod/go-version/jkaninda/posta)](https://go.dev/)
[![Go Reference](https://pkg.go.dev/badge/github.com/jkaninda/posta.svg)](https://pkg.go.dev/github.com/jkaninda/posta)
[![GitHub Release](https://img.shields.io/github/v/release/jkaninda/posta)](https://github.com/jkaninda/posta/releases)
![Docker Image Size (latest by date)](https://img.shields.io/docker/image-size/jkaninda/posta?style=flat-square)
![Docker Pulls](https://img.shields.io/docker/pulls/jkaninda/posta?style=flat-square)

**Posta** is a self-hosted email delivery platform that allows applications to send emails through HTTP APIs while Posta manages SMTP delivery, templates, storage, security, and analytics.

It provides a developer-friendly and fully self-hostable alternative to services such as SendGrid, and Mailgun.

# Send Your First Email

Example request:

```bash
curl -X POST http://localhost:9000/api/v1/emails/send \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "from": "hello@example.com",
    "to": ["user@example.com"],
    "subject": "Hello from Posta",
    "html": "<h1>Hello!</h1>"
  }'
```

Response:

```json
{
  "id": "email_01J8C8E5W3",
  "status": "queued"
}
```

---

# Features

## Email Delivery

* **HTTP Email API**
  Send single, batch, or template-based emails via REST endpoints.

* **Scheduled Delivery**
  Queue emails for delivery at a specified time.

* **Asynchronous Processing**
  Background workers process email queues using Redis and Asynq with priority tiers (transactional, bulk, and low-priority).

* **Automatic Retry**
  Failed emails are retried automatically with configurable retry limits per SMTP server.

* **Development Mode**
  Store and preview emails in the dashboard without sending them.

---

## Templates

* **Versioned Templates**
  Create multiple template versions with a selectable active version.

* **Multi-language Support**
  Templates support language-specific versions with variable substitution.

* **Managed Stylesheets**
  Stylesheets are automatically inlined for email client compatibility.

* **Template Preview**
  Render and preview templates directly from the dashboard.

* **Template Import / Export**
  Export templates as JSON and import them across environments.

---

## SMTP and Domain Management

* **Multiple SMTP Servers**
  Configure multiple SMTP servers per user with SSL or STARTTLS support.

* **Shared SMTP Pool**
  Administrators can define shared SMTP servers available to all users.

* **Domain Verification**
  Verify domain ownership via DNS records including SPF, DKIM, and DMARC.

* **Verified Sending Enforcement**
  Optionally restrict sending to verified domains only.

---

## Security and Authentication

* **API Key Authentication**
  Secure API keys with hashing, prefix identification, expiration, IP allowlisting, and revocation.

* **Dashboard Authentication**
  JWT-based authentication with role-based access control.

* **Two-Factor Authentication**
  TOTP-based 2FA setup and verification.

* **Rate Limiting**
  Redis-backed hourly and daily email limits per user.

---

## Contacts and Suppression

* **Contact Management**
  Automatically track recipients with send and failure statistics.

* **Contact Lists**
  Organize recipients into reusable mailing lists.

* **Bounce Tracking**
  Track hard bounces, soft bounces, and complaints.

* **Automatic Suppression**
  Automatically suppress recipients based on bounce behavior.

---

## Events and Webhooks

* **Webhooks**
  Subscribe to events such as `email.sent` and `email.failed`.

* **Audit Logs**
  Track platform and user activity with filtering and real-time streaming using Server-Sent Events.

---

## Analytics and Monitoring

* **Email Analytics**
  View daily email volume and status breakdown with date filtering.

* **Dashboard Statistics**
  Overview of delivery metrics and recent activity.

* **Prometheus Metrics**
  Export metrics including request counts, latencies, and email delivery counters.

* **Health Probes**
  Liveness (`/healthz`) and readiness (`/readyz`) endpoints.

---

## Admin Panel

* **User Management**
  Create, deactivate, and manage users and roles.

* **Platform Metrics**
  Aggregate statistics across the entire platform.

* **Shared SMTP Servers**
  Manage SMTP servers available to all users.

* **Platform Email Logs**
  View and search emails across all users.

* **Job Monitoring**
  Track scheduled cron jobs such as retention cleanup and reports.

* **Platform Settings**
  Configure registration, retention policies, and bounce handling.

* **Real-time Event Streaming**
  Live audit log updates via Server-Sent Events.

---

## Dashboard

* **Vue.js Web Interface**
  Manage templates, SMTP servers, domains, API keys, contacts, webhooks, and email logs.

* **User Settings**
  Configure timezone, default sender, notification preferences, API key expiration, bounce handling, and daily reports.

---

## API Documentation

* **Swagger UI** — `/docs`
* **ReDoc** — `/redoc`

---

# Tech Stack

**Frontend**

- Framework: Vue 3 with Composition API
- Build Tool: Vite
- State Management: Pinia
- HTTP Client: Axios

**Backend**

- Language: Go
- Framework: [Okapi](https://github.com/jkaninda/okapi)
- Database: PostgreSQL
- Queue: Redis with [Asynq](https://github.com/hibiken/asynq)
- Metrics:               Prometheus-compatible
---

# Requirements

* Go 1.25+
* PostgreSQL
* Redis

---

# Quick Start

## Local Development

```bash
git clone https://github.com/jkaninda/posta.git
cd posta

make dev-deps
make dev
make dev-worker
```

---

## Docker Compose

```bash
docker compose up -d
```

This starts:

* Posta API server
* Background worker
* PostgreSQL
* Redis

Dashboard:

```
http://localhost:9000
```

Default admin credentials:

```
Email: admin@example.com
Password: admin1234
```

---
# Dashboard

Posta includes a web dashboard for managing templates, SMTP servers, domains, contacts, API keys, and analytics.

<p align="center">
  <img src="https://raw.githubusercontent.com/jkaninda/posta/main/docs/screenshots/dashboard.png" alt="Posta Dashboard" width="900"/>
</p>

### Email Analytics

<p align="center">
  <img src="https://raw.githubusercontent.com/jkaninda/posta/main/docs/screenshots/analytics.png" alt="Email Analytics" width="900"/>
</p>

### Email Logs

<p align="center">
  <img src="https://raw.githubusercontent.com/jkaninda/posta/main/docs/screenshots/email-logs.png" alt="Email Logs" width="900"/>
</p>

### Email Detail

<p align="center">
  <img src="https://raw.githubusercontent.com/jkaninda/posta/main/docs/screenshots/email-detail.png" alt="Email Detail" width="900"/>
</p>

### Template Detail

<p align="center">
  <img src="https://raw.githubusercontent.com/jkaninda/posta/main/docs/screenshots/template-detail.png" alt="Template Detail" width="900"/>
</p>

### Template Preview

<p align="center">
  <img src="https://raw.githubusercontent.com/jkaninda/posta/main/docs/screenshots/template-preview.png" alt="Template Preview" width="900"/>
</p>

### Admin Platform Metrics

<p align="center">
  <img src="https://raw.githubusercontent.com/jkaninda/posta/main/docs/screenshots/admin-platform-metrics.png" alt="Admin Platform Metrics" width="900"/>
</p>

### Admin Metrics (Light)

<p align="center">
  <img src="https://raw.githubusercontent.com/jkaninda/posta/main/docs/screenshots/admin-metrics-light.png" alt="Admin Metrics Light" width="900"/>
</p>

### Admin Metrics (Dark)

<p align="center">
  <img src="https://raw.githubusercontent.com/jkaninda/posta/main/docs/screenshots/admin-metrics-dark.png" alt="Admin Metrics Dark" width="900"/>
</p>

---
## Official Clients

- Go: https://github.com/jkaninda/posta-go
- Php: https://github.com/jkaninda/posta-php
- Java: https://github.com/jkaninda/posta-java

### Go Client SDK

An official Go client is available:


Install:

```bash
go get github.com/jkaninda/posta-go
```

Example:

```go
package main

import (
    "fmt"
    "log"

    posta "github.com/jkaninda/posta-go"
)

func main() {
    client := posta.New("https://posta.example.com", "your-api-key")

    resp, err := client.SendEmail(&posta.SendEmailRequest{
        From:    "sender@example.com",
        To:      []string{"recipient@example.com"},
        Subject: "Hello from Posta",
        HTML:    "<h1>Hello!</h1>",
    })

    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Email sent: id=%s status=%s\n", resp.ID, resp.Status)
}
```

---

## Contributing

Contributions are welcome! Please open an issue to discuss proposed changes before submitting a pull request.

## License

Copyright 2025 Jonas Kaninda

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License. You may obtain a copy of the License at

```
http://www.apache.org/licenses/LICENSE-2.0
```
---

<div align="center">

**Made with ❤️ for the developer community**

⭐ **Star us on GitHub** — it motivates us to keep improving!

Copyright © 2026 Jonas Kaninda

</div>