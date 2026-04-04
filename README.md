# Posta

<p align="center">
  <img src="https://raw.githubusercontent.com/goposta/posta/main/logo.png" alt="Posta" width="150" />
</p>

<p align="center">
  Self-hosted email delivery platform for developers and teams
</p>

[![CI](https://github.com/goposta/posta/actions/workflows/ci.yml/badge.svg)](https://github.com/goposta/posta/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/goposta/posta)](https://goreportcard.com/report/github.com/goposta/posta)
[![Go](https://img.shields.io/github/go-mod/go-version/goposta/posta)](https://go.dev/)
[![Go Reference](https://pkg.go.dev/badge/github.com/goposta/posta.svg)](https://pkg.go.dev/github.com/goposta/posta)
[![GitHub Release](https://img.shields.io/github/v/release/goposta/posta)](https://github.com/goposta/posta/releases)
![Docker Image Size (latest by date)](https://img.shields.io/docker/image-size/jkaninda/posta?style=flat-square)
![Docker Pulls](https://img.shields.io/docker/pulls/jkaninda/posta?style=flat-square)




---

## Overview

**Posta** is a self-hosted email delivery platform that enables applications to send emails via HTTP APIs while handling SMTP delivery, templates, storage, security, and analytics.

It is designed as a developer-first, fully self-hostable alternative to services like SendGrid or Mailgun.

[![Website](https://img.shields.io/badge/Website-goposta.dev-blue?style=flat-square)](https://www.goposta.dev/)
[![Try it](https://img.shields.io/badge/Try%20it-app.goposta.dev-green?style=flat-square)](https://app.goposta.dev/)
---

## Quick Example

Send your first email:

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

## Core Features

### Email Delivery

* REST API for transactional, batch, and templated emails
* Attachments, custom headers, and unsubscribe support
* Scheduled sending and preview mode
* Async processing with Redis and Asynq
* Automatic retries and priority queues

### Templates

* Versioned and multi-language templates
* Variable substitution and stylesheet inlining
* Import/export and preview support

### SMTP & Domains

* Multiple SMTP providers with TLS support
* Shared SMTP pools for teams
* Domain verification (SPF, DKIM, DMARC)
* Verified sender enforcement

### Security

* API keys with expiration, hashing, and IP allowlisting
* JWT authentication and RBAC
* Two-factor authentication (TOTP)
* OAuth / SSO login (Google, Keycloak, authentik, and more)
* Rate limiting and session management

### Contacts & Suppression

* Contact tracking and segmentation
* Bounce and complaint handling
* Automatic suppression lists

### Workspaces

* Multi-tenant architecture with isolated workspaces
* Role-based access control
* Shared resources and scoped API keys

### Webhooks & Events

* Event-driven architecture with webhook delivery
* Retry strategies and delivery tracking
* Audit logs and real-time event streaming

### Analytics & Monitoring

* Email delivery metrics and trends
* Prometheus integration
* Health endpoints and daily reports

### Admin Platform

* User and API key management
* Global metrics and logs
* SMTP pool management
* Platform configuration and retention policies

### Dashboard

* Vue-based UI for managing all resources
* Analytics, templates, SMTP, contacts, and logs
* Dark/light mode and user preferences

---

## Architecture

* Backend: Go (Okapi framework)
* Frontend: Vue 3 + Vite
* Database: PostgreSQL
* Queue: Redis + Asynq
* Metrics: Prometheus

---

## Requirements

* Go 1.25+
* PostgreSQL
* Redis

---

## Quick Start

### Docker Compose

```bash
docker compose up -d
```

Access the dashboard:

```
http://localhost:9000
```

Default credentials:

```
Email: admin@example.com
Password: admin1234
```

---

### Local Development

```bash
git clone https://github.com/goposta/posta.git
cd posta

make dev-deps
make dev
make dev-worker
```

---

## API Documentation

* Swagger UI: `/docs`
* ReDoc: `/redoc`
---

# Dashboard

Posta includes a web dashboard for managing templates, SMTP servers, domains, contacts, API keys, and analytics.

<p align="center">
  <img src="https://raw.githubusercontent.com//goposta/posta/main/docs/static/img/screenshots/dashboard.png" alt="Posta Dashboard" width="900"/>
</p>

### Email Analytics

<p align="center">
  <img src="https://raw.githubusercontent.com//goposta/posta/main/docs/static/img/screenshots/analytics.png" alt="Email Analytics" width="900"/>
</p>

### Template Detail

<p align="center">
  <img src="https://raw.githubusercontent.com//goposta/posta/main/docs/static/img/screenshots/template-detail.png" alt="Template Detail" width="900"/>
</p>

### Template Editor

<p align="center">
  <img src="https://raw.githubusercontent.com//goposta/posta/main/docs/static/img/screenshots/template-editor.png" alt="Template Editor" width="900"/>
</p>

### Admin Platform Metrics

<p align="center">
  <img src="https://raw.githubusercontent.com//goposta/posta/main/docs/static/img/screenshots/admin-platform-metrics.png" alt="Admin Platform Metrics" width="900"/>
</p>

### Admin Platform Metrics (Dark)

<p align="center">
  <img src="https://raw.githubusercontent.com//goposta/posta/main/docs/static/img/screenshots/admin-platform-metrics-dark.png" alt="Admin Platform Metrics Dark" width="900"/>
</p>

---

## Official SDKs

* Go: [https://github.com/goposta/posta-go](https://github.com/goposta/posta-go)
* PHP: [https://github.com/goposta/posta-php](https://github.com/goposta/posta-php)
* Java: [https://github.com/goposta/posta-java](https://github.com/goposta/posta-java)

### Go Example

```go
client := posta.New("https://posta.example.com", "your-api-key")

resp, err := client.SendEmail(&posta.SendEmailRequest{
    From:    "sender@example.com",
    To:      []string{"recipient@example.com"},
    Subject: "Hello from Posta",
    HTML:    "<h1>Hello!</h1>",
})
```

---

## Contributing

Contributions are welcome. Please open an issue before submitting a pull request.

---

## License

Apache License 2.0

## Copyright

Copyright (c) 2026 Jonas Kaninda and contributors

